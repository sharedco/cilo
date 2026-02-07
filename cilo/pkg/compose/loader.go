package compose

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type ServiceMeta struct {
	Name   string
	Labels map[string]string
}

// LoadServices reads compose files and returns merged service metadata.
// Later files override earlier ones for labels.
func LoadServices(files []string) (map[string]*ServiceMeta, error) {
	services := map[string]*ServiceMeta{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read compose file %s: %w", file, err)
		}

		var root map[string]interface{}
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("failed to parse compose file %s: %w", file, err)
		}

		rawServices, ok := root["services"]
		if !ok {
			continue
		}
		servicesMap, ok := rawServices.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid services section in %s", file)
		}

		for name, rawSvc := range servicesMap {
			meta := services[name]
			if meta == nil {
				meta = &ServiceMeta{Name: name, Labels: map[string]string{}}
				services[name] = meta
			}

			svcMap, ok := rawSvc.(map[string]interface{})
			if !ok {
				continue
			}
			if labels, ok := svcMap["labels"]; ok {
				for k, v := range normalizeLabels(labels) {
					meta.Labels[k] = v
				}
			}
		}
	}

	return services, nil
}

// ResolveComposeFiles returns absolute compose file paths and the project directory.
func ResolveComposeFiles(workspace string, composeFiles []string) ([]string, string, error) {
	if len(composeFiles) == 0 {
		composeFiles = []string{"docker-compose.yml"}
	}

	files := make([]string, 0, len(composeFiles))
	for _, f := range composeFiles {
		path := f
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspace, f)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, "", fmt.Errorf("compose file not found: %s", path)
		}
		files = append(files, path)
	}

	projectDir := filepath.Dir(files[0])
	return files, projectDir, nil
}

func normalizeLabels(labels interface{}) map[string]string {
	result := map[string]string{}
	if labels == nil {
		return result
	}

	switch val := labels.(type) {
	case map[string]interface{}:
		for k, v := range val {
			result[k] = fmt.Sprintf("%v", v)
		}
	case map[interface{}]interface{}:
		for k, v := range val {
			result[fmt.Sprintf("%v", k)] = fmt.Sprintf("%v", v)
		}
	case []interface{}:
		for _, item := range val {
			label := fmt.Sprintf("%v", item)
			parts := strings.SplitN(label, "=", 2)
			if len(parts) == 2 {
				result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	case []string:
		for _, label := range val {
			parts := strings.SplitN(label, "=", 2)
			if len(parts) == 2 {
				result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	return result
}

func SortedServiceNames(services map[string]*ServiceMeta) []string {
	if len(services) == 0 {
		return nil
	}
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetServicesWithLabel returns service names that have a specific label with the given value
func GetServicesWithLabel(composeFiles []string, labelKey string, labelValue string) ([]string, error) {
	services, err := LoadServices(composeFiles)
	if err != nil {
		return nil, err
	}

	var result []string
	for name, meta := range services {
		if meta.Labels != nil {
			if value, ok := meta.Labels[labelKey]; ok && value == labelValue {
				result = append(result, name)
			}
		}
	}

	sort.Strings(result)
	return result, nil
}
