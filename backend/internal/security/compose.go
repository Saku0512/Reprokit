// Package security verifies that a Docker Compose document stays within the
// Reprokit v0.1 runtime boundary before it can be used to start a Lab.
package security

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	digestImagePattern  = regexp.MustCompile(`^[^\s@]+@sha256:[a-f0-9]{64}$`)
	serviceNamePattern  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	resourceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	portPattern         = regexp.MustCompile(`^(?:(127\.0\.0\.1):)?([0-9]{1,5}):([0-9]{1,5})$`)
	volumePattern       = regexp.MustCompile(`^([a-zA-Z0-9][a-zA-Z0-9_.-]*):(/[^:]+?)(?::(ro|rw))?$`)
)

// Inspection is the accepted Compose document in the form the runtime must
// persist and start. Compose inputs that omit a host address are normalized to
// 127.0.0.1 so a caller cannot accidentally publish them externally.
type Inspection struct {
	Normalized []byte
}

// Guardrails はレビュー済みLabに固定する実行上限である。利用者入力や
// Compose入力から任意に緩和できないよう、Manifestからのみ渡される。
type Guardrails struct {
	MaxRuntimeSeconds int    `yaml:"maxRuntimeSeconds" json:"maxRuntimeSeconds"`
	CPUCores          int    `yaml:"cpuCores" json:"cpuCores"`
	MemoryMiB         int    `yaml:"memoryMiB" json:"memoryMiB"`
	PIDs              int    `yaml:"pids" json:"pids"`
	DiskMiB           int    `yaml:"diskMiB" json:"diskMiB"`
	Egress            string `yaml:"egress" json:"egress"`
}

// Validate はホスト上のDockerが確実に扱える保守的な範囲だけを許可する。
func (guardrails Guardrails) Validate() error {
	if guardrails.MaxRuntimeSeconds < 60 || guardrails.MaxRuntimeSeconds > 4*60*60 {
		return errors.New("maxRuntimeSeconds must be between 60 and 14400")
	}
	if guardrails.CPUCores < 1 || guardrails.CPUCores > 4 {
		return errors.New("cpuCores must be between 1 and 4")
	}
	if guardrails.MemoryMiB < 128 || guardrails.MemoryMiB > 4096 {
		return errors.New("memoryMiB must be between 128 and 4096")
	}
	if guardrails.PIDs < 64 || guardrails.PIDs > 512 {
		return errors.New("pids must be between 64 and 512")
	}
	if guardrails.DiskMiB < 256 || guardrails.DiskMiB > 8192 {
		return errors.New("diskMiB must be between 256 and 8192")
	}
	if guardrails.Egress != "none" {
		return errors.New("egress must be none")
	}
	return nil
}

// Violation identifies a forbidden setting and the action needed to correct
// it. Service is "<compose>" for a top-level setting.
type Violation struct {
	Service     string
	Setting     string
	Reason      string
	Remediation string
}

func (violation Violation) Error() string {
	return fmt.Sprintf("service %q: setting %q: %s; fix: %s", violation.Service, violation.Setting, violation.Reason, violation.Remediation)
}

// ValidationError contains every issue found in a Compose document. The
// runtime must treat this error as a hard start blocker.
type ValidationError struct {
	Violations []Violation
}

func (err *ValidationError) Error() string {
	parts := make([]string, len(err.Violations))
	for index, violation := range err.Violations {
		parts[index] = violation.Error()
	}
	return "Compose security validation failed: " + strings.Join(parts, "; ")
}

// InspectCompose strictly parses a single Compose YAML document, rejects
// unsupported or unsafe features, and returns a safe normalized document. A
// successful inspection is required but not sufficient to start a Lab: the
// runtime must also verify its lockfile and source hashes.
func InspectCompose(contents []byte) (Inspection, error) {
	root, err := parseCompose(contents)
	if err != nil {
		return Inspection{}, err
	}

	validator := composeValidator{}
	validator.validateSyntax(root)
	validator.validateRoot(root)
	if len(validator.violations) != 0 {
		return Inspection{}, &ValidationError{Violations: validator.violations}
	}

	normalized, err := yaml.Marshal(root)
	if err != nil {
		return Inspection{}, fmt.Errorf("encode normalized Compose: %w", err)
	}
	return Inspection{Normalized: normalized}, nil
}

// InspectComposeWithGuardrails は安全性検査済みComposeに、Manifestで固定した
// 実行上限と外向き通信を持たない専用ネットワークを付与する。入力Composeに
// deployやstorage_optを許可するものではない。
func InspectComposeWithGuardrails(contents []byte, guardrails Guardrails) (Inspection, error) {
	if err := guardrails.Validate(); err != nil {
		return Inspection{}, fmt.Errorf("validate guardrails: %w", err)
	}
	root, err := parseCompose(contents)
	if err != nil {
		return Inspection{}, err
	}
	validator := composeValidator{}
	validator.validateSyntax(root)
	validator.validateRoot(root)
	if len(validator.violations) != 0 {
		return Inspection{}, &ValidationError{Violations: validator.violations}
	}
	if err := applyGuardrails(root, guardrails); err != nil {
		return Inspection{}, err
	}
	normalized, err := yaml.Marshal(root)
	if err != nil {
		return Inspection{}, fmt.Errorf("encode normalized Compose: %w", err)
	}
	return Inspection{Normalized: normalized}, nil
}

// ValidateCompose is useful where only the pass/fail decision is needed. A
// caller that will persist or execute the Compose configuration must use the
// Normalized bytes returned by InspectCompose instead of the original input.
func ValidateCompose(contents []byte) error {
	_, err := InspectCompose(contents)
	return err
}

func parseCompose(contents []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("parse Compose YAML: %w", err)
	}
	if document.Kind == 0 || len(document.Content) != 1 {
		return nil, errors.New("Compose must contain exactly one non-empty YAML document")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not allowed")
		}
		return nil, fmt.Errorf("read Compose YAML: %w", err)
	}
	return document.Content[0], nil
}

type composeValidator struct {
	violations []Violation
}

func (validator *composeValidator) add(service, setting, reason, remediation string) {
	validator.violations = append(validator.violations, Violation{
		Service:     service,
		Setting:     setting,
		Reason:      reason,
		Remediation: remediation,
	})
}

func (validator *composeValidator) validateSyntax(node *yaml.Node) {
	if node.Anchor != "" {
		validator.add("<compose>", "YAML anchor", "YAML anchors are not supported", "inline the value without anchors or aliases")
	}
	if node.Kind == yaml.AliasNode {
		validator.add("<compose>", "YAML alias", "YAML aliases are not supported", "inline the value without aliases or merge keys")
	}
	for _, child := range node.Content {
		validator.validateSyntax(child)
	}
}

func (validator *composeValidator) validateRoot(root *yaml.Node) {
	if root.Kind != yaml.MappingNode {
		validator.add("<compose>", "document", "Compose root must be a mapping", "use a mapping with services and optional volumes or networks")
		return
	}
	allowed := map[string]bool{"version": true, "services": true, "volumes": true, "networks": true}
	fields := mapping(root, validator, "<compose>", "top-level")
	for _, key := range sortedKeys(fields) {
		if !allowed[key] {
			validator.add("<compose>", key, "unsupported top-level Compose option", "remove it; v0.1 permits only version, services, volumes, and networks")
		}
	}
	if version, found := fields["version"]; found && !isScalar(version) {
		validator.add("<compose>", "version", "version must be a scalar", "use an optional legacy scalar version or remove it")
	}
	services, found := fields["services"]
	if !found {
		validator.add("<compose>", "services", "services is required", "declare at least one image-pinned service")
		return
	}
	if services.Kind != yaml.MappingNode || len(services.Content) == 0 {
		validator.add("<compose>", "services", "services must be a non-empty mapping", "declare at least one named service")
		return
	}
	volumes := validator.validateResources(fields["volumes"], "volumes")
	networks := validator.validateResources(fields["networks"], "networks")
	serviceFields := mapping(services, validator, "<compose>", "services")
	serviceNames := make([]string, 0, len(serviceFields))
	for name := range serviceFields {
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames)
	for _, name := range serviceNames {
		if !serviceNamePattern.MatchString(name) {
			validator.add(name, "service name", "service name is invalid", "use letters, digits, dots, underscores, or hyphens")
		}
		validator.validateService(name, serviceFields[name], volumes, networks, serviceFields)
	}
	validator.validateDependencyCycles(serviceFields)
}

func (validator *composeValidator) validateResources(node *yaml.Node, setting string) map[string]bool {
	resources := map[string]bool{}
	if node == nil {
		return resources
	}
	if node.Kind != yaml.MappingNode {
		validator.add("<compose>", setting, setting+" must be a mapping", "declare only project-owned named "+setting)
		return resources
	}
	fields := mapping(node, validator, "<compose>", setting)
	for _, name := range sortedKeys(fields) {
		definition := fields[name]
		if !resourceNamePattern.MatchString(name) {
			validator.add("<compose>", setting, fmt.Sprintf("%s name %q is invalid", setting, name), "use a project-owned simple name")
			continue
		}
		resources[name] = true
		if definition.Kind != yaml.MappingNode || len(definition.Content) != 0 {
			validator.add("<compose>", setting, fmt.Sprintf("%s %q has unsupported options", setting, name), "declare it as an empty mapping; external names, drivers, and options are forbidden")
		}
	}
	return resources
}

func (validator *composeValidator) validateService(name string, node *yaml.Node, volumes, networks map[string]bool, services map[string]*yaml.Node) {
	if node.Kind != yaml.MappingNode {
		validator.add(name, "service", "service definition must be a mapping", "declare an image-pinned service mapping")
		return
	}
	allowed := map[string]bool{"image": true, "ports": true, "environment": true, "depends_on": true, "volumes": true, "networks": true, "restart": true}
	fields := mapping(node, validator, name, "service")
	for _, setting := range sortedKeys(fields) {
		if allowed[setting] {
			continue
		}
		validator.addForbiddenServiceSetting(name, setting)
	}
	image, found := fields["image"]
	if !found || !isString(image) || !digestImagePattern.MatchString(image.Value) {
		validator.add(name, "image", "image must be pinned to a sha256 digest", "use image: registry/name@sha256:<64 lowercase hex characters>")
	}
	if restart, found := fields["restart"]; found && (!isString(restart) || restart.Value != "no") {
		validator.add(name, "restart", "automatic restart is not allowed", "remove restart or set restart: \"no\"")
	}
	validator.validatePorts(name, fields["ports"])
	validator.validateEnvironment(name, fields["environment"])
	validator.validateDependsOn(name, fields["depends_on"], services)
	validator.validateVolumes(name, fields["volumes"], volumes)
	validator.validateNetworks(name, fields["networks"], networks)
}

func (validator *composeValidator) addForbiddenServiceSetting(service, setting string) {
	reason, remediation := "unsupported service option", "remove the option; only the documented v0.1 allowlist is accepted"
	switch setting {
	case "privileged":
		reason, remediation = "privileged containers are forbidden", "remove privileged: true"
	case "network_mode":
		reason, remediation = "host or custom network modes are forbidden", "remove network_mode and use a declared project Bridge network"
	case "pid", "ipc":
		reason, remediation = "host PID or IPC namespaces are forbidden", "remove the setting"
	case "cap_add", "cap_drop":
		reason, remediation = "Linux capability changes are forbidden", "remove capability configuration"
	case "devices", "device_cgroup_rules":
		reason, remediation = "host device access is forbidden", "remove device configuration"
	case "build":
		reason, remediation = "build contexts and Dockerfiles are forbidden", "publish a reviewed digest-pinned image and use image instead"
	case "env_file", "configs", "secrets", "include", "extends":
		reason, remediation = "external Compose inputs are forbidden", "inline reviewed fixed values in the allowed fields"
	case "command", "entrypoint":
		reason, remediation = "container command overrides are forbidden", "use the reviewed image default command"
	}
	validator.add(service, setting, reason, remediation)
}

func (validator *composeValidator) validatePorts(service string, node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind != yaml.SequenceNode {
		validator.add(service, "ports", "ports must be a list of fixed TCP strings", "use \"127.0.0.1:HOST:CONTAINER\"")
		return
	}
	for _, port := range node.Content {
		if !isString(port) {
			validator.add(service, "ports", "port entries must be strings", "use \"127.0.0.1:HOST:CONTAINER\"")
			continue
		}
		matches := portPattern.FindStringSubmatch(port.Value)
		if matches == nil || !validPort(matches[2]) || !validPort(matches[3]) {
			validator.add(service, "ports", fmt.Sprintf("port entry %q is not a fixed TCP host-to-container mapping", port.Value), "use \"127.0.0.1:HOST:CONTAINER\" with ports from 1 through 65535")
			continue
		}
		if matches[1] == "" {
			port.Value = "127.0.0.1:" + port.Value
		}
	}
}

func (validator *composeValidator) validateEnvironment(service string, node *yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind != yaml.MappingNode {
		validator.add(service, "environment", "environment must be a mapping of fixed strings", "use literal KEY: VALUE entries or remove it")
		return
	}
	values := mapping(node, validator, service, "environment")
	for _, key := range sortedKeys(values) {
		value := values[key]
		if !isString(value) || strings.Contains(value.Value, "$") {
			validator.add(service, "environment", fmt.Sprintf("environment value for %q must be a fixed string", key), "replace it with a literal value without variable interpolation")
		}
	}
}

func (validator *composeValidator) validateDependsOn(service string, node *yaml.Node, services map[string]*yaml.Node) {
	if node == nil {
		return
	}
	if node.Kind != yaml.SequenceNode {
		validator.add(service, "depends_on", "depends_on must be a short-form list", "use a list of declared service names")
		return
	}
	for _, dependency := range node.Content {
		if !isString(dependency) || services[dependency.Value] == nil {
			validator.add(service, "depends_on", "depends_on references an unknown or invalid service", "use only declared service names in short form")
		}
	}
}

func (validator *composeValidator) validateVolumes(service string, node *yaml.Node, volumes map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind != yaml.SequenceNode {
		validator.add(service, "volumes", "volumes must be a list of named-volume mounts", "use declared-name:/container/path[:ro|rw]")
		return
	}
	for _, volume := range node.Content {
		if !isString(volume) {
			validator.add(service, "volumes", "volume entries must be strings", "use declared-name:/container/path[:ro|rw]")
			continue
		}
		if strings.Contains(strings.ToLower(volume.Value), "docker.sock") {
			validator.add(service, "volumes", "Docker socket mounts are forbidden", "remove the Docker or Containerd socket mount")
			continue
		}
		matches := volumePattern.FindStringSubmatch(volume.Value)
		if matches == nil || strings.Contains(matches[2], "..") || !volumes[matches[1]] {
			validator.add(service, "volumes", fmt.Sprintf("volume entry %q is not a declared named-volume mount", volume.Value), "remove host bind mounts and use a declared named volume at an absolute container path")
		}
	}
}

func (validator *composeValidator) validateNetworks(service string, node *yaml.Node, networks map[string]bool) {
	if node == nil {
		return
	}
	if node.Kind != yaml.SequenceNode || len(node.Content) > 1 {
		validator.add(service, "networks", "networks must contain at most one declared project Bridge network", "use one declared network name or omit networks")
		return
	}
	for _, network := range node.Content {
		if !isString(network) || !networks[network.Value] {
			validator.add(service, "networks", "network is not a declared project Bridge network", "use a declared network name or omit networks")
		}
	}
}

func (validator *composeValidator) validateDependencyCycles(services map[string]*yaml.Node) {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string)
	visit = func(service string) {
		if visiting[service] {
			validator.add(service, "depends_on", "dependency cycle is not allowed", "remove the cycle")
			return
		}
		if visited[service] {
			return
		}
		visited[service] = true
		visiting[service] = true
		fields := mapping(services[service], validator, service, "service")
		if dependencies := fields["depends_on"]; dependencies != nil && dependencies.Kind == yaml.SequenceNode {
			for _, dependency := range dependencies.Content {
				if isString(dependency) && services[dependency.Value] != nil {
					visit(dependency.Value)
				}
			}
		}
		delete(visiting, service)
	}
	for _, service := range sortedKeys(services) {
		visit(service)
	}
}

func mapping(node *yaml.Node, validator *composeValidator, service, setting string) map[string]*yaml.Node {
	fields := map[string]*yaml.Node{}
	if node == nil || node.Kind != yaml.MappingNode {
		return fields
	}
	for index := 0; index < len(node.Content); index += 2 {
		key, value := node.Content[index], node.Content[index+1]
		if !isString(key) {
			validator.add(service, setting, "mapping keys must be strings", "use a string key")
			continue
		}
		if _, duplicate := fields[key.Value]; duplicate {
			validator.add(service, key.Value, "duplicate key is not allowed", "keep exactly one value for the setting")
			continue
		}
		fields[key.Value] = value
	}
	return fields
}

func isScalar(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode
}

func isString(node *yaml.Node) bool {
	return isScalar(node) && node.Tag == "!!str"
}

func validPort(value string) bool {
	port, err := strconv.Atoi(value)
	return err == nil && port >= 1 && port <= 65535
}

func applyGuardrails(root *yaml.Node, guardrails Guardrails) error {
	fields := rawMapping(root)
	services := fields["services"]
	if services == nil || services.Kind != yaml.MappingNode {
		return errors.New("Compose services are missing after validation")
	}
	const networkName = "reprokit_isolated"
	setMapping(root, "networks", mappingNode(map[string]*yaml.Node{
		networkName: mappingNode(map[string]*yaml.Node{"internal": boolNode(true)}),
	}))
	serviceFields := rawMapping(services)
	for _, name := range sortedKeys(serviceFields) {
		service := serviceFields[name]
		if service.Kind != yaml.MappingNode {
			return fmt.Errorf("service %q is invalid after validation", name)
		}
		setMapping(service, "networks", sequenceNode(stringNode(networkName)))
		setMapping(service, "deploy", mappingNode(map[string]*yaml.Node{
			"resources": mappingNode(map[string]*yaml.Node{
				"limits": mappingNode(map[string]*yaml.Node{
					"cpus":   stringNode(strconv.Itoa(guardrails.CPUCores)),
					"memory": stringNode(strconv.Itoa(guardrails.MemoryMiB) + "M"),
					"pids":   intNode(guardrails.PIDs),
				}),
			}),
		}))
		setMapping(service, "storage_opt", mappingNode(map[string]*yaml.Node{
			"size": stringNode(strconv.Itoa(guardrails.DiskMiB) + "M"),
		}))
	}
	return nil
}

func rawMapping(node *yaml.Node) map[string]*yaml.Node {
	fields := map[string]*yaml.Node{}
	if node == nil || node.Kind != yaml.MappingNode {
		return fields
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if key := node.Content[index]; isString(key) {
			fields[key.Value] = node.Content[index+1]
		}
	}
	return fields
}

func setMapping(node *yaml.Node, key string, value *yaml.Node) {
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			node.Content[index+1] = value
			return
		}
	}
	node.Content = append(node.Content, stringNode(key), value)
}

func mappingNode(fields map[string]*yaml.Node) *yaml.Node {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, key := range sortedKeys(fields) {
		node.Content = append(node.Content, stringNode(key), fields[key])
	}
	return node
}

func sequenceNode(values ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Content: values}
}

func stringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func intNode(value int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(value)}
}

func boolNode(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}
}

func sortedKeys(values map[string]*yaml.Node) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
