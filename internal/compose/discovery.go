package compose

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"docker-panel/internal/models"
)

type DiscoveredStack struct {
	Name           string
	Path           string
	ComposePath    string
	ComposeContent string
	EnvContent     string
}

func DiscoverStacks(stacksRoot string) ([]DiscoveredStack, error) {
	if stacksRoot == "" {
		stacksRoot = "/srv/docker-panel/stacks"
	}
	stacksRoot = filepath.Clean(stacksRoot)

	entries, err := os.ReadDir(stacksRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var results []DiscoveredStack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stackName := entry.Name()
		if strings.HasPrefix(stackName, ".") {
			continue
		}

		dir := filepath.Join(stacksRoot, stackName)
		composeCandidates := []string{
			"compose.yaml",
			"compose.yml",
			"docker-compose.yaml",
			"docker-compose.yml",
		}

		var chosenCompose string
		for _, candidate := range composeCandidates {
			p := filepath.Join(dir, candidate)
			if info, err := os.Stat(p); err == nil && !info.IsDir() {
				chosenCompose = p
				break
			}
		}

		if chosenCompose == "" {
			continue
		}

		cBytes, err := os.ReadFile(chosenCompose)
		if err != nil {
			continue
		}

		envFile := filepath.Join(dir, ".env")
		eBytes, _ := os.ReadFile(envFile)

		results = append(results, DiscoveredStack{
			Name:           stackName,
			Path:           dir,
			ComposePath:    chosenCompose,
			ComposeContent: string(cBytes),
			EnvContent:     string(eBytes),
		})
	}

	return results, nil
}

func MatchContainersToStacks(stacks []*models.Stack, containers []models.ContainerInfo) []*models.Stack {
	stackMap := make(map[string]*models.Stack)
	for _, s := range stacks {
		s.Containers = make([]models.ContainerInfo, 0)
		stackMap[s.Name] = s
	}

	for _, c := range containers {
		if c.StackName != "" {
			st, ok := stackMap[c.StackName]
			if !ok {
				workingDir := c.WorkingDir
				if workingDir == "" {
					workingDir = c.Labels["com.docker.compose.project.working_dir"]
				}
				configFile := c.ComposeFile
				if configFile == "" {
					configFile = c.Labels["com.docker.compose.project.config_files"]
				}
				st = &models.Stack{
					ID:            "stk_" + c.StackName,
					Name:          c.StackName,
					Status:        models.StackStatusStopped,
					Path:          workingDir,
					WorkingDir:    workingDir,
					ComposeFile:   configFile,
					IsSystem:      c.StackName == "docker-panel",
					SecurityScore: 0,
					Containers:    make([]models.ContainerInfo, 0),
					CreatedAt:     time.Unix(c.Created, 0),
					UpdatedAt:     time.Now(),
				}
				stackMap[c.StackName] = st
				stacks = append(stacks, st)
			}
			if st.WorkingDir == "" {
				st.WorkingDir = c.WorkingDir
			}
			if st.Path == "" {
				st.Path = c.WorkingDir
			}
			if st.ComposeFile == "" {
				st.ComposeFile = c.ComposeFile
			}
			st.Containers = append(st.Containers, c)
		}
	}

	for _, s := range stacks {
		if len(s.Containers) == 0 {
			s.Status = models.StackStatusStopped
			continue
		}
		runningCount := 0
		for _, c := range s.Containers {
			if strings.EqualFold(c.State, "running") {
				runningCount++
			}
		}
		if runningCount == len(s.Containers) {
			s.Status = models.StackStatusRunning
		} else if runningCount > 0 {
			s.Status = models.StackStatusPartial
		} else {
			s.Status = models.StackStatusStopped
		}
	}
	return stacks
}
