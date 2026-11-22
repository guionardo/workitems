package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const responseLimit = 4 << 20

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "workitems-setup:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("provider is required: github-projects or gitlab-board")
	}
	switch args[0] {
	case "github-projects":
		return runGitHub(args[1:])
	case "gitlab-board":
		return runGitLab(args[1:])
	default:
		return fmt.Errorf("unsupported provider %q", args[0])
	}
}

func runGitHub(args []string) error {
	flags := flag.NewFlagSet("github-projects", flag.ContinueOnError)
	tokenEnv := flags.String("token-env", "GITHUB_API_TOKEN", "environment variable containing the GitHub token")
	owner := flags.String("owner", os.Getenv("GITHUB_OWNER"), "GitHub user or organization login")
	ownerType := flags.String("owner-type", "auto", "GitHub owner type: auto, user, or organization")
	projectNumber := flags.Int("project-number", envInt("GITHUB_PROJECT_NUMBER"), "GitHub Projects V2 number")
	output := flags.String("output", "", "environment file to write; stdout when empty")
	force := flags.Bool("force", false, "overwrite an existing output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv(*tokenEnv))
	if token == "" || strings.TrimSpace(*owner) == "" || *projectNumber < 1 {
		return errors.New("GitHub token, owner, and a positive project number are required")
	}
	project, err := discoverGitHubProject(token, *owner, *projectNumber, *ownerType)
	if err != nil {
		return err
	}
	var status struct {
		ID      string
		Options map[string]string
	}
	for _, field := range project.Fields.Nodes {
		if field.Name != "Status" {
			continue
		}
		status.ID = field.ID
		status.Options = make(map[string]string, len(field.Options))
		for _, option := range field.Options {
			status.Options[strings.ToLower(strings.ReplaceAll(option.Name, " ", "_"))] = option.ID
		}
	}
	if status.ID == "" {
		return errors.New("GitHub project has no single-select Status field")
	}
	lines := []string{
		"GITHUB_API_TOKEN=",
		"GITHUB_OWNER=" + *owner,
		"GITHUB_PROJECT_NUMBER=" + strconv.Itoa(project.Number),
		"GITHUB_PROJECT_ID=" + project.ID,
		"GITHUB_STATUS_FIELD_ID=" + status.ID,
	}
	for _, name := range []string{"todo", "in_progress", "done"} {
		if id := status.Options[name]; id != "" {
			lines = append(lines, "GITHUB_STATUS_"+strings.ToUpper(name)+"_OPTION_ID="+id)
		}
	}
	return writeOutput(*output, *force, lines)
}

type githubProject struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Fields struct {
		Nodes []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Options []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"options"`
		} `json:"nodes"`
	} `json:"fields"`
}

func discoverGitHubProject(token, owner string, number int, ownerType string) (githubProject, error) {
	if ownerType != "auto" && ownerType != "user" && ownerType != "organization" {
		return githubProject{}, errors.New("GitHub owner type must be auto, user, or organization")
	}
	for _, kind := range ownerTypes(ownerType) {
		query := `query($login:String!, $number:Int!) { ` + kind + `(login:$login) { projectV2(number:$number) { id number fields(first:100) { nodes { ... on ProjectV2SingleSelectField { id name options { id name } } } } } } }`
		var response struct {
			Data struct {
				User struct {
					Project githubProject `json:"projectV2"`
				} `json:"user"`
				Organization struct {
					Project githubProject `json:"projectV2"`
				} `json:"organization"`
			} `json:"data"`
		}
		if err := graphql("https://api.github.com/graphql", token, query, map[string]any{"login": owner, "number": number}, &response); err != nil {
			return githubProject{}, err
		}
		project := response.Data.User.Project
		if kind == "organization" {
			project = response.Data.Organization.Project
		}
		if project.ID != "" {
			return project, nil
		}
	}
	return githubProject{}, errors.New("GitHub project was not found for the selected owner type")
}

func ownerTypes(ownerType string) []string {
	if ownerType == "auto" {
		return []string{"user", "organization"}
	}
	return []string{ownerType}
}

func runGitLab(args []string) error {
	flags := flag.NewFlagSet("gitlab-board", flag.ContinueOnError)
	tokenEnv := flags.String("token-env", "GITLAB_API_TOKEN", "environment variable containing the GitLab token")
	host := flags.String("host", valueOr("GITLAB_HOST", "https://gitlab.com"), "GitLab host")
	project := flags.String("project", os.Getenv("GITLAB_PROJECT"), "GitLab project ID or path")
	boardID := flags.Int("board-id", envInt("GITLAB_BOARD_ID"), "GitLab board ID; first board when omitted")
	output := flags.String("output", "", "environment file to write; stdout when empty")
	force := flags.Bool("force", false, "overwrite an existing output file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	token := strings.TrimSpace(os.Getenv(*tokenEnv))
	if token == "" || strings.TrimSpace(*project) == "" {
		return errors.New("GitLab token and project are required")
	}
	client := &apiClient{baseURL: strings.TrimRight(*host, "/"), token: token}
	projectPath := url.PathEscape(*project)
	var boards []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := client.get("/api/v4/projects/"+projectPath+"/boards", &boards); err != nil {
		return err
	}
	if *boardID == 0 {
		if len(boards) == 0 {
			return errors.New("GitLab project has no issue boards")
		}
		*boardID = boards[0].ID
	}
	var lists []struct {
		ID    int `json:"id"`
		Label *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"label"`
	}
	if err := client.get(fmt.Sprintf("/api/v4/projects/%s/boards/%d/lists", projectPath, *boardID), &lists); err != nil {
		return err
	}
	lines := []string{"GITLAB_API_TOKEN=", "GITLAB_HOST=" + strings.TrimRight(*host, "/"), "GITLAB_PROJECT=" + *project, "GITLAB_BOARD_ID=" + strconv.Itoa(*boardID)}
	for _, list := range lists {
		if list.Label == nil {
			continue
		}
		key := strings.ToUpper(strings.ReplaceAll(list.Label.Name, "-", "_"))
		lines = append(lines, "GITLAB_BOARD_"+key+"_LABEL_ID="+strconv.Itoa(list.Label.ID), "GITLAB_BOARD_"+key+"_LIST_ID="+strconv.Itoa(list.ID))
	}
	return writeOutput(*output, *force, lines)
}

type apiClient struct {
	baseURL string
	token   string
}

func (c *apiClient) get(endpoint string, target any) error {
	request, err := http.NewRequest(http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("PRIVATE-TOKEN", c.token)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GitLab request failed with HTTP status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil {
		return err
	}
	if len(data) > responseLimit {
		return errors.New("provider response exceeds size limit")
	}
	return json.Unmarshal(data, target)
}

func graphql(endpoint, token, query string, variables map[string]any, target any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("GitHub request failed with HTTP status %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, responseLimit+1))
	if err != nil {
		return err
	}
	if len(data) > responseLimit {
		return errors.New("provider response exceeds size limit")
	}
	return json.Unmarshal(data, target)
}

func writeOutput(path string, force bool, lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	if path == "" {
		_, err := fmt.Print(content)
		return err
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("output file %q already exists; use --force", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "wrote %s\n", path)
	return nil
}

func envInt(name string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	return value
}

func valueOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
