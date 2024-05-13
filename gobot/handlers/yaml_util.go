package handlers

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type SkillYaml struct {
	Task_description string `yaml:"task_description"`
	Created_by       string `yaml:"created_by"`
	Seed_examples    []struct {
		Question yaml.Node
		Context  yaml.Node
		Answer   yaml.Node
	} `yaml:"seed_examples"`
}

type KnowledgeYaml struct {
	Task_description string `yaml:"task_description"`
	Created_by       string `yaml:"created_by"`
	Domain           string `yaml:"domain"`
	Seed_examples    []struct {
		Question yaml.Node
		Answer   yaml.Node
	} `yaml:"seed_examples"`
	Document struct {
		Repo     string   `yaml:"repo"`
		Commit   string   `yaml:"commit"`
		Patterns []string `yaml:"patterns"`
	} `yaml:"document"`
}

func (prc *PullRequestCreateHandler) generateKnowledgeYaml(requestData KnowledgePRRequest) (string, error) {
	knowledgeYaml := KnowledgeYaml{
		Task_description: strings.TrimSpace(requestData.Task_description),
		Created_by:       strings.TrimSpace(requestData.Name),
		Domain:           strings.TrimSpace(requestData.Domain),
		Seed_examples: []struct {
			Question yaml.Node
			Answer   yaml.Node
		}{},
		Document: struct {
			Repo     string   `yaml:"repo"`
			Commit   string   `yaml:"commit"`
			Patterns []string `yaml:"patterns"`
		}{
			Repo:     strings.TrimSpace(requestData.Repo),
			Commit:   strings.TrimSpace(requestData.Commit),
			Patterns: strings.Split(strings.TrimSpace(requestData.Patterns), ","),
		},
	}

	for i, question := range requestData.Questions {
		knowledgeYaml.Seed_examples = append(knowledgeYaml.Seed_examples, struct {
			Question yaml.Node
			Answer   yaml.Node
		}{
			yaml.Node{
				Kind:  yaml.ScalarNode,
				Style: yaml.FoldedStyle,
				Value: strings.TrimSpace(question),
			},
			yaml.Node{
				Kind:  yaml.ScalarNode,
				Style: yaml.FoldedStyle,
				Value: strings.TrimSpace(requestData.Answers[i]),
			},
		})
	}

	// Generate the yaml file using new yaml encoder
	var buf bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&buf)
	err := yamlEncoder.Encode(knowledgeYaml)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (prc *PullRequestCreateHandler) generateKnowledgeAttributionData(requestData KnowledgePRRequest) string {
	return fmt.Sprintf("Title of work: %s \nLink to work: %s \nRevision: %s \nLicense of the work: %s \nCreator names: %s",
		strings.TrimSpace(requestData.Title_work), strings.TrimSpace(requestData.Link_work),
		strings.TrimSpace(requestData.Revision), strings.TrimSpace(requestData.License_work), strings.TrimSpace(requestData.Creators))
}

func (prc *PullRequestCreateHandler) generateSkillYaml(requestData SkillPRRequest) (string, error) {
	skillYaml := SkillYaml{
		Task_description: strings.TrimSpace(requestData.Task_description),
		Created_by:       strings.TrimSpace(requestData.Name),
		Seed_examples: []struct {
			Question yaml.Node
			Context  yaml.Node
			Answer   yaml.Node
		}{},
	}

	for i, question := range requestData.Questions {
		skillYaml.Seed_examples = append(skillYaml.Seed_examples, struct {
			Question yaml.Node
			Context  yaml.Node
			Answer   yaml.Node
		}{
			yaml.Node{
				Kind:  yaml.ScalarNode,
				Style: yaml.FoldedStyle,
				Value: strings.TrimSpace(question),
			},
			yaml.Node{
				Kind:  yaml.ScalarNode,
				Style: yaml.FoldedStyle,
				Value: strings.TrimSpace(requestData.Contexts[i]),
			},
			yaml.Node{
				Kind:  yaml.ScalarNode,
				Style: yaml.FoldedStyle,
				Value: strings.TrimSpace(requestData.Answers[i]),
			},
		})
	}

	// Generate the yaml file using new yaml encoder
	var buf bytes.Buffer
	yamlEncoder := yaml.NewEncoder(&buf)
	err := yamlEncoder.Encode(skillYaml)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (prc *PullRequestCreateHandler) generateSkillAttributionData(requestData SkillPRRequest) string {
	return fmt.Sprintf("Title of work: %s \nLink to work: %s \nLicense of the work: %s \nCreator names: %s",
		strings.TrimSpace(requestData.Title_work), strings.TrimSpace(requestData.Link_work),
		strings.TrimSpace(requestData.License_work), strings.TrimSpace(requestData.Creators))
}
