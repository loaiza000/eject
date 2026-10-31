package compatibility

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/loaiza000/eject/internal/domain"
)

var (
	imagePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]*$`)
	variablePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	metadataPattern = regexp.MustCompile(`^[A-Za-z0-9._:+-]*$`)
	hostPattern     = regexp.MustCompile(`^[A-Za-z0-9.-]*$`)
)

// ValidateSource limits the metadata embedded in a plan or generation.
func ValidateSource(infra domain.Infrastructure) error {
	if len(infra.Resources) > 10000 || len(infra.Dependencies) > 20000 || len(infra.Findings) > 20000 {
		return errors.New("source entry count exceeds limit")
	}
	for _, r := range infra.Resources {
		if len(r.Properties) > 1024 {
			return errors.New("resource property count exceeds limit")
		}
		for key, value := range r.Properties {
			switch r.Kind {
			case "compute.container":
				switch {
				case key == "image":
					if !imagePattern.MatchString(value) {
						return fmt.Errorf("unsafe image reference for %q", r.ID)
					}
				case strings.HasPrefix(key, "port_"):
					port, protocol, ok := strings.Cut(value, "/")
					n, err := strconv.Atoi(port)
					if !ok || err != nil || n < 1 || n > 65535 || (protocol != "tcp" && protocol != "udp") {
						return fmt.Errorf("unsafe port for %q", r.ID)
					}
				case strings.HasPrefix(key, "env_name_") || strings.HasPrefix(key, "secret_name_"):
					if !variablePattern.MatchString(value) {
						return fmt.Errorf("unsafe variable name for %q", r.ID)
					}
				default:
					return fmt.Errorf("unrecognized container property %q", key)
				}
			case "database":
				if key != "engine" && key != "engine_version" && key != "endpoint" && key != "port" {
					return fmt.Errorf("unrecognized database property %q", key)
				}
				if key == "endpoint" {
					if !hostPattern.MatchString(value) {
						return fmt.Errorf("unsafe database endpoint for %q", r.ID)
					}
				} else if !metadataPattern.MatchString(value) {
					return fmt.Errorf("unsafe database property for %q", r.ID)
				}
			case "compute.task":
				if key != "network_mode" {
					return fmt.Errorf("unrecognized task property %q", key)
				}
				if !metadataPattern.MatchString(value) {
					return fmt.Errorf("unsafe task property for %q", r.ID)
				}
			case "object_storage":
				if key != "versioning" && key != "mfa_delete" && !strings.HasPrefix(key, "encryption_") {
					return fmt.Errorf("unrecognized storage property %q", key)
				}
				if !metadataPattern.MatchString(value) {
					return fmt.Errorf("unsafe storage property for %q", r.ID)
				}
			default:
				if len(r.Properties) > 0 {
					return fmt.Errorf("unexpected properties for %q", r.ID)
				}
			}
		}
	}
	return nil
}
