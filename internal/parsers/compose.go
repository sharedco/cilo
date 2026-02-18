// Copyright (c) 2026 Cilo Authors
// SPDX-License-Identifier: MIT
// See LICENSES/MIT.txt for full license text

package parsers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sharedco/cilo/internal/engine"
	"gopkg.in/yaml.v3"
)

// ComposeParser parses Docker Compose files into EnvironmentSpec
type ComposeParser struct{}

// Name returns the parser name
func (p *ComposeParser) Name() string {
	return "compose"
}

// Detect checks if the project has compose files
func (p *ComposeParser) Detect(projectPath string) bool {
	// Check for compose files in order of priority
	candidates := []string{
		"compose.yml",
		"compose.yaml",
		"docker-compose.yml",
		"docker-compose.yaml",
	}

	for _, name := range candidates {
		if _, err := os.Stat(filepath.Join(projectPath, name)); err == nil {
			return true
		}
	}
	return false
}

// Parse reads compose files and returns an EnvironmentSpec
func (p *ComposeParser) Parse(projectPath string) (*engine.EnvironmentSpec, error) {
	// 1. Find compose files
	composeFiles, err := findComposeFiles(projectPath)
	if err != nil {
		return nil, err
	}

	// 2. Parse and merge compose files
	services, networks, volumes, err := parseComposeFiles(composeFiles)
	if err != nil {
		return nil, err
	}

	// 3. Convert to EnvironmentSpec
	spec := &engine.EnvironmentSpec{
		Source:     "compose",
		SourcePath: composeFiles[0],
		Services:   make([]engine.ServiceSpec, 0, len(services)),
		Networks:   make([]engine.NetworkSpec, 0, len(networks)),
		Volumes:    make([]engine.VolumeSpec, 0, len(volumes)),
	}

	// Convert services
	for name, svc := range services {
		serviceSpec := convertComposeService(name, svc)
		spec.Services = append(spec.Services, serviceSpec)
	}

	// Convert networks
	for name, net := range networks {
		networkSpec := convertComposeNetwork(name, net)
		spec.Networks = append(spec.Networks, networkSpec)
	}

	// Convert volumes
	for name, vol := range volumes {
		volumeSpec := convertComposeVolume(name, vol)
		spec.Volumes = append(spec.Volumes, volumeSpec)
	}

	return spec, nil
}

// findComposeFiles locates compose files in order of priority
func findComposeFiles(projectPath string) ([]string, error) {
	candidates := []string{
		"compose.yml",
		"compose.yaml",
		"docker-compose.yml",
		"docker-compose.yaml",
	}

	var found []string
	for _, name := range candidates {
		path := filepath.Join(projectPath, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
			break // Use first found file
		}
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("no compose file found in %s", projectPath)
	}

	return found, nil
}

// composeFile represents the structure of a docker-compose.yml file
type composeFile struct {
	Version  string                            `yaml:"version"`
	Services map[string]*composeService        `yaml:"services"`
	Networks map[string]*composeNetwork        `yaml:"networks"`
	Volumes  map[string]map[string]interface{} `yaml:"volumes"`
}

// composeService represents a service in a docker-compose file
type composeService struct {
	Image           string                 `yaml:"image"`
	Build           interface{}            `yaml:"build"`
	Command         interface{}            `yaml:"command"`
	Entrypoint      interface{}            `yaml:"entrypoint"`
	Environment     interface{}            `yaml:"environment"`
	Ports           []interface{}          `yaml:"ports"`
	Volumes         []interface{}          `yaml:"volumes"`
	DependsOn       interface{}            `yaml:"depends_on"`
	Labels          interface{}            `yaml:"labels"`
	Networks        interface{}            `yaml:"networks"`
	WorkingDir      string                 `yaml:"working_dir"`
	User            string                 `yaml:"user"`
	Hostname        string                 `yaml:"hostname"`
	Restart         string                 `yaml:"restart"`
	Privileged      bool                   `yaml:"privileged"`
	CapAdd          []string               `yaml:"cap_add"`
	CapDrop         []string               `yaml:"cap_drop"`
	DNS             interface{}            `yaml:"dns"`
	DNSSearch       interface{}            `yaml:"dns_search"`
	ExtraHosts      []string               `yaml:"extra_hosts"`
	EnvFile         interface{}            `yaml:"env_file"`
	Tmpfs           interface{}            `yaml:"tmpfs"`
	ShmSize         interface{}            `yaml:"shm_size"`
	StopSignal      string                 `yaml:"stop_signal"`
	StopGracePeriod interface{}            `yaml:"stop_grace_period"`
	SecurityOpt     []string               `yaml:"security_opt"`
	Ulimits         map[string]interface{} `yaml:"ulimits"`
	Sysctls         interface{}            `yaml:"sysctls"`
	HealthCheck     *composeHealthCheck    `yaml:"healthcheck"`
	NetworkMode     string                 `yaml:"network_mode"`
}

// composeHealthCheck represents a health check configuration
type composeHealthCheck struct {
	Test        interface{} `yaml:"test"`
	Interval    string      `yaml:"interval"`
	Timeout     string      `yaml:"timeout"`
	Retries     int         `yaml:"retries"`
	StartPeriod string      `yaml:"start_period"`
	Disable     bool        `yaml:"disable"`
}

// composeNetwork represents a network in a docker-compose file
type composeNetwork struct {
	Driver     string            `yaml:"driver"`
	DriverOpts map[string]string `yaml:"driver_opts"`
	IPAM       *composeIPAM      `yaml:"ipam"`
	Internal   bool              `yaml:"internal"`
	Attachable bool              `yaml:"attachable"`
	Labels     map[string]string `yaml:"labels"`
	EnableIPv6 bool              `yaml:"enable_ipv6"`
	External   interface{}       `yaml:"external"`
}

// composeIPAM represents IPAM configuration
type composeIPAM struct {
	Driver  string                   `yaml:"driver"`
	Config  []map[string]interface{} `yaml:"config"`
	Options map[string]string        `yaml:"options"`
}

// parseComposeFiles parses and merges multiple compose files
func parseComposeFiles(paths []string) (map[string]*composeService, map[string]*composeNetwork, map[string]map[string]interface{}, error) {
	services := make(map[string]*composeService)
	networks := make(map[string]*composeNetwork)
	volumes := make(map[string]map[string]interface{})

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to read compose file %s: %w", path, err)
		}

		var cf composeFile
		if err := yaml.Unmarshal(data, &cf); err != nil {
			return nil, nil, nil, fmt.Errorf("failed to parse compose file %s: %w", path, err)
		}

		// Merge services (later files override earlier)
		for name, svc := range cf.Services {
			services[name] = svc
		}

		// Merge networks
		for name, net := range cf.Networks {
			networks[name] = net
		}

		// Merge volumes
		for name, vol := range cf.Volumes {
			volumes[name] = vol
		}
	}

	return services, networks, volumes, nil
}

// convertComposeService converts a ComposeService to ServiceSpec
func convertComposeService(name string, svc *composeService) engine.ServiceSpec {
	spec := engine.ServiceSpec{
		Name:        name,
		Image:       svc.Image,
		WorkingDir:  svc.WorkingDir,
		User:        svc.User,
		Hostname:    svc.Hostname,
		Restart:     svc.Restart,
		Privileged:  svc.Privileged,
		CapAdd:      svc.CapAdd,
		CapDrop:     svc.CapDrop,
		StopSignal:  svc.StopSignal,
		SecurityOpt: svc.SecurityOpt,
		NetworkMode: svc.NetworkMode,
		ExtraHosts:  svc.ExtraHosts,
	}

	// Convert build
	if svc.Build != nil {
		spec.Build = convertBuild(svc.Build)
	}

	// Convert command
	if svc.Command != nil {
		spec.Command = convertStringOrArray(svc.Command)
	}

	// Convert entrypoint
	if svc.Entrypoint != nil {
		spec.Entrypoint = convertStringOrArray(svc.Entrypoint)
	}

	// Convert environment
	if svc.Environment != nil {
		spec.Env = convertEnvironment(svc.Environment)
	}

	// Convert env_file
	if svc.EnvFile != nil {
		spec.EnvFile = convertStringOrArray(svc.EnvFile)
	}

	// Convert ports
	spec.Ports = convertPorts(svc.Ports)

	// Convert volumes
	spec.Volumes = convertVolumes(svc.Volumes)

	// Convert depends_on
	spec.DependsOn = convertDependsOn(svc.DependsOn)

	// Convert labels
	spec.Labels = convertLabels(svc.Labels)

	// Convert DNS
	if svc.DNS != nil {
		spec.DNS = convertStringOrArray(svc.DNS)
	}

	// Convert DNS search
	if svc.DNSSearch != nil {
		spec.DNSSearch = convertStringOrArray(svc.DNSSearch)
	}

	// Convert tmpfs
	if svc.Tmpfs != nil {
		spec.Tmpfs = convertStringOrArray(svc.Tmpfs)
	}

	// Convert shm_size
	if svc.ShmSize != nil {
		spec.ShmSize = fmt.Sprintf("%v", svc.ShmSize)
	}

	// Convert stop_grace_period
	if svc.StopGracePeriod != nil {
		spec.StopGracePeriod = fmt.Sprintf("%v", svc.StopGracePeriod)
	}

	// Convert ulimits
	spec.Ulimits = convertUlimits(svc.Ulimits)

	// Convert sysctls
	if svc.Sysctls != nil {
		spec.Sysctls = convertSysctls(svc.Sysctls)
	}

	// Convert health check
	if svc.HealthCheck != nil {
		spec.HealthCheck = convertHealthCheck(svc.HealthCheck)
	}

	// Convert networks
	spec.Networks = convertNetworks(svc.Networks)

	return spec
}

// convertBuild converts build configuration
func convertBuild(build interface{}) *engine.BuildSpec {
	if build == nil {
		return nil
	}

	switch b := build.(type) {
	case string:
		return &engine.BuildSpec{Context: b}
	case map[string]interface{}:
		spec := &engine.BuildSpec{
			Context:    getString(b, "context"),
			Dockerfile: getString(b, "dockerfile"),
			Target:     getString(b, "target"),
			Network:    getString(b, "network"),
		}

		// Convert args
		if args, ok := b["args"]; ok {
			spec.Args = convertStringMap(args)
		}

		// Convert cache_from
		if cacheFrom, ok := b["cache_from"]; ok {
			spec.CacheFrom = convertStringOrArray(cacheFrom)
		}

		// Convert labels
		if labels, ok := b["labels"]; ok {
			spec.Labels = convertStringMap(labels)
		}

		// Convert shm_size
		if shmSize, ok := b["shm_size"]; ok {
			spec.ShmSize = fmt.Sprintf("%v", shmSize)
		}

		return spec
	}

	return nil
}

// convertPorts converts port specifications
func convertPorts(ports []interface{}) []engine.PortSpec {
	if len(ports) == 0 {
		return nil
	}

	result := make([]engine.PortSpec, 0, len(ports))
	for _, p := range ports {
		switch port := p.(type) {
		case string:
			// Short syntax: "8080:80" or "127.0.0.1:8080:80/tcp"
			spec := parsePortString(port)
			if spec != nil {
				result = append(result, *spec)
			}
		case int:
			// Just a port number (container port only)
			result = append(result, engine.PortSpec{
				Target:   port,
				Protocol: "tcp",
			})
		case map[string]interface{}:
			// Long syntax
			spec := engine.PortSpec{
				Protocol: getStringWithDefault(port, "protocol", "tcp"),
			}
			if target, ok := port["target"]; ok {
				spec.Target = toInt(target)
			}
			if published, ok := port["published"]; ok {
				spec.Published = toInt(published)
			}
			if hostIP, ok := port["host_ip"]; ok {
				spec.HostIP = fmt.Sprintf("%v", hostIP)
			}
			result = append(result, spec)
		}
	}

	return result
}

// parsePortString parses a port string like "8080:80" or "127.0.0.1:8080:80/tcp"
func parsePortString(port string) *engine.PortSpec {
	spec := &engine.PortSpec{Protocol: "tcp"}

	// Check for protocol suffix
	parts := strings.Split(port, "/")
	if len(parts) == 2 {
		spec.Protocol = parts[1]
		port = parts[0]
	}

	// Parse host:container or host_ip:host:container
	portParts := strings.Split(port, ":")
	switch len(portParts) {
	case 1:
		// Just container port
		spec.Target = toInt(portParts[0])
	case 2:
		// host:container
		spec.Published = toInt(portParts[0])
		spec.Target = toInt(portParts[1])
	case 3:
		// host_ip:host:container
		spec.HostIP = portParts[0]
		spec.Published = toInt(portParts[1])
		spec.Target = toInt(portParts[2])
	default:
		return nil
	}

	return spec
}

// convertVolumes converts volume specifications
func convertVolumes(volumes []interface{}) []engine.VolumeMountSpec {
	if len(volumes) == 0 {
		return nil
	}

	result := make([]engine.VolumeMountSpec, 0, len(volumes))
	for _, v := range volumes {
		switch vol := v.(type) {
		case string:
			// Short syntax: "./data:/app/data:ro"
			spec := parseVolumeString(vol)
			if spec != nil {
				result = append(result, *spec)
			}
		case map[string]interface{}:
			// Long syntax
			spec := engine.VolumeMountSpec{
				Type:     getStringWithDefault(vol, "type", "volume"),
				Source:   getString(vol, "source"),
				Target:   getString(vol, "target"),
				ReadOnly: getBool(vol, "read_only"),
			}

			// Convert bind options
			if bind, ok := vol["bind"]; ok {
				if bindMap, ok := bind.(map[string]interface{}); ok {
					spec.Bind = &engine.BindOptions{
						Propagation:    getString(bindMap, "propagation"),
						CreateHostPath: getBool(bindMap, "create_host_path"),
					}
				}
			}

			// Convert volume options
			if volume, ok := vol["volume"]; ok {
				if volumeMap, ok := volume.(map[string]interface{}); ok {
					spec.Volume = &engine.VolumeOptions{
						NoCopy: getBool(volumeMap, "nocopy"),
					}
				}
			}

			// Convert tmpfs options
			if tmpfs, ok := vol["tmpfs"]; ok {
				if tmpfsMap, ok := tmpfs.(map[string]interface{}); ok {
					spec.Tmpfs = &engine.TmpfsOptions{
						Size: toInt64(tmpfsMap["size"]),
						Mode: toInt(tmpfsMap["mode"]),
					}
				}
			}

			result = append(result, spec)
		}
	}

	return result
}

// parseVolumeString parses a volume string like "./data:/app/data:ro"
func parseVolumeString(volume string) *engine.VolumeMountSpec {
	parts := strings.Split(volume, ":")

	spec := &engine.VolumeMountSpec{
		Type: "bind",
	}

	switch len(parts) {
	case 1:
		// Just a volume name or target
		spec.Target = parts[0]
		spec.Type = "volume"
	case 2:
		// source:target
		spec.Source = parts[0]
		spec.Target = parts[1]
		// Determine type based on source
		if strings.HasPrefix(parts[0], ".") || strings.HasPrefix(parts[0], "/") {
			spec.Type = "bind"
		} else {
			spec.Type = "volume"
		}
	case 3:
		// source:target:options
		spec.Source = parts[0]
		spec.Target = parts[1]
		if strings.Contains(parts[2], "ro") {
			spec.ReadOnly = true
		}
		// Determine type
		if strings.HasPrefix(parts[0], ".") || strings.HasPrefix(parts[0], "/") {
			spec.Type = "bind"
		} else {
			spec.Type = "volume"
		}
	default:
		return nil
	}

	return spec
}

// convertEnvironment converts environment variables
func convertEnvironment(env interface{}) map[string]string {
	result := make(map[string]string)

	switch e := env.(type) {
	case map[string]interface{}:
		for k, v := range e {
			result[k] = fmt.Sprintf("%v", v)
		}
	case []interface{}:
		// List format: ["FOO=bar", "BAZ=qux"]
		for _, item := range e {
			str := fmt.Sprintf("%v", item)
			parts := strings.SplitN(str, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			} else {
				result[parts[0]] = ""
			}
		}
	}

	return result
}

// convertDependsOn converts depends_on
func convertDependsOn(deps interface{}) []string {
	if deps == nil {
		return nil
	}

	switch d := deps.(type) {
	case []interface{}:
		// List format: ["db", "redis"]
		result := make([]string, 0, len(d))
		for _, item := range d {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	case []string:
		return d
	case map[string]interface{}:
		// Map format: { db: { condition: service_healthy } }
		result := make([]string, 0, len(d))
		for name := range d {
			result = append(result, name)
		}
		return result
	}

	return nil
}

// convertLabels converts labels
func convertLabels(labels interface{}) map[string]string {
	if labels == nil {
		return nil
	}

	result := make(map[string]string)

	switch l := labels.(type) {
	case map[string]interface{}:
		for k, v := range l {
			result[k] = fmt.Sprintf("%v", v)
		}
	case map[string]string:
		return l
	case []interface{}:
		// List format: ["label1=value1", "label2=value2"]
		for _, item := range l {
			str := fmt.Sprintf("%v", item)
			parts := strings.SplitN(str, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}

	return result
}

// convertHealthCheck converts health check configuration
func convertHealthCheck(hc *composeHealthCheck) *engine.HealthCheckSpec {
	if hc == nil {
		return nil
	}

	spec := &engine.HealthCheckSpec{
		Interval:    hc.Interval,
		Timeout:     hc.Timeout,
		Retries:     hc.Retries,
		StartPeriod: hc.StartPeriod,
		Disable:     hc.Disable,
	}

	// Convert test
	if hc.Test != nil {
		spec.Test = convertStringOrArray(hc.Test)
	}

	return spec
}

// convertNetworks converts network configuration for a service
func convertNetworks(networks interface{}) []string {
	if networks == nil {
		return nil
	}

	switch n := networks.(type) {
	case []interface{}:
		result := make([]string, 0, len(n))
		for _, item := range n {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	case []string:
		return n
	case map[string]interface{}:
		// Map format with network names as keys
		result := make([]string, 0, len(n))
		for name := range n {
			result = append(result, name)
		}
		return result
	}

	return nil
}

// convertComposeNetwork converts a compose network to NetworkSpec
func convertComposeNetwork(name string, net *composeNetwork) engine.NetworkSpec {
	if net == nil {
		return engine.NetworkSpec{Name: name}
	}

	spec := engine.NetworkSpec{
		Name:       name,
		Driver:     net.Driver,
		DriverOpts: net.DriverOpts,
		Internal:   net.Internal,
		Attachable: net.Attachable,
		Labels:     net.Labels,
		EnableIPv6: net.EnableIPv6,
	}

	// Convert IPAM
	if net.IPAM != nil {
		spec.IPAM = &engine.IPAMSpec{
			Driver:  net.IPAM.Driver,
			Options: net.IPAM.Options,
		}

		if len(net.IPAM.Config) > 0 {
			spec.IPAM.Config = make([]engine.IPAMConfig, 0, len(net.IPAM.Config))
			for _, cfg := range net.IPAM.Config {
				ipamConfig := engine.IPAMConfig{
					Subnet:  getString(cfg, "subnet"),
					IPRange: getString(cfg, "ip_range"),
					Gateway: getString(cfg, "gateway"),
				}

				// Convert aux_addresses
				if auxAddr, ok := cfg["aux_addresses"]; ok {
					if auxMap, ok := auxAddr.(map[string]interface{}); ok {
						ipamConfig.AuxAddresses = make(map[string]string)
						for k, v := range auxMap {
							ipamConfig.AuxAddresses[k] = fmt.Sprintf("%v", v)
						}
					}
				}

				spec.IPAM.Config = append(spec.IPAM.Config, ipamConfig)
			}
		}
	}

	return spec
}

// convertComposeVolume converts a compose volume to VolumeSpec
func convertComposeVolume(name string, vol map[string]interface{}) engine.VolumeSpec {
	if vol == nil {
		return engine.VolumeSpec{Name: name}
	}

	spec := engine.VolumeSpec{
		Name:     name,
		Driver:   getString(vol, "driver"),
		External: getBool(vol, "external"),
	}

	// Convert driver_opts
	if driverOpts, ok := vol["driver_opts"]; ok {
		spec.DriverOpts = convertStringMap(driverOpts)
	}

	// Convert labels
	if labels, ok := vol["labels"]; ok {
		spec.Labels = convertStringMap(labels)
	}

	return spec
}

// convertUlimits converts ulimits
func convertUlimits(ulimits map[string]interface{}) map[string]engine.UlimitSpec {
	if len(ulimits) == 0 {
		return nil
	}

	result := make(map[string]engine.UlimitSpec)
	for name, value := range ulimits {
		switch v := value.(type) {
		case int:
			result[name] = engine.UlimitSpec{Soft: int64(v), Hard: int64(v)}
		case map[string]interface{}:
			result[name] = engine.UlimitSpec{
				Soft: toInt64(v["soft"]),
				Hard: toInt64(v["hard"]),
			}
		}
	}

	return result
}

// convertSysctls converts sysctls
func convertSysctls(sysctls interface{}) map[string]string {
	if sysctls == nil {
		return nil
	}

	switch s := sysctls.(type) {
	case map[string]interface{}:
		return convertStringMap(s)
	case map[string]string:
		return s
	}

	return nil
}

// Helper functions

func convertStringOrArray(value interface{}) []string {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		// Handle shell-style command parsing
		return []string{v}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			result = append(result, fmt.Sprintf("%v", item))
		}
		return result
	case []string:
		return v
	}

	return nil
}

func convertStringMap(value interface{}) map[string]string {
	if value == nil {
		return nil
	}

	result := make(map[string]string)

	switch v := value.(type) {
	case map[string]interface{}:
		for k, val := range v {
			result[k] = fmt.Sprintf("%v", val)
		}
	case map[string]string:
		return v
	}

	return result
}

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		return fmt.Sprintf("%v", val)
	}
	return ""
}

func getStringWithDefault(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		return fmt.Sprintf("%v", val)
	}
	return defaultValue
}

func getBool(m map[string]interface{}, key string) bool {
	if val, ok := m[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

func toInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	}
	return 0
}

func toInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	}
	return 0
}
