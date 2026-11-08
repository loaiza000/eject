package aws

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/loaiza000/eject/internal/domain"
)

func parse(file *os.File, e entry, out *domain.Infrastructure) error {
	switch e.Operation {
	case "ecs.describe-task-definition":
		return parseTask(file, e, out)
	case "ecs.describe-services":
		return parseServices(file, e, out)
	case "rds.describe-db-instances":
		return parseRDS(file, e, out)
	case "s3.get-bucket-versioning":
		return parseVersioning(file, e, out)
	case "s3.get-bucket-encryption":
		return parseEncryption(file, e, out)
	default:
		return fmt.Errorf("unsupported operation %q", e.Operation)
	}
}

func source(e entry) string                       { return e.Operation + ":" + e.File }
func resourceID(kind, region, name string) string { return "aws:" + region + ":" + kind + ":" + name }

func parseTask(file *os.File, e entry, out *domain.Infrastructure) error {
	var response struct {
		TaskDefinition struct {
			TaskDefinitionArn    string `json:"taskDefinitionArn"`
			Family               string `json:"family"`
			NetworkMode          string `json:"networkMode"`
			ContainerDefinitions []struct {
				Name         string `json:"name"`
				Image        string `json:"image"`
				PortMappings []struct {
					ContainerPort int    `json:"containerPort"`
					Protocol      string `json:"protocol"`
				} `json:"portMappings"`
				Environment []struct {
					Name string `json:"name"`
				} `json:"environment"`
				Secrets []struct {
					Name string `json:"name"`
				} `json:"secrets"`
			} `json:"containerDefinitions"`
		} `json:"taskDefinition"`
	}
	if err := readJSON(file, &response, false); err != nil {
		return err
	}
	t := response.TaskDefinition
	if t.TaskDefinitionArn == "" || t.Family == "" || len(t.ContainerDefinitions) == 0 {
		return errors.New("taskDefinitionArn, family and containerDefinitions are required")
	}
	if e.Resource != t.TaskDefinitionArn {
		return errors.New("manifest resource does not match taskDefinitionArn")
	}
	id := resourceID("task", e.Region, t.TaskDefinitionArn)
	out.Resources = append(out.Resources, domain.Resource{ID: id, Kind: "compute.task", Provider: "aws", Name: t.Family, Properties: map[string]string{"network_mode": t.NetworkMode}, Sources: []string{source(e)}, Status: "DISCOVERED"})
	for _, c := range t.ContainerDefinitions {
		if c.Name == "" || c.Image == "" {
			return errors.New("container name and image are required")
		}
		properties := map[string]string{"image": c.Image}
		for j, p := range c.PortMappings {
			if p.ContainerPort < 1 || p.ContainerPort > 65535 {
				return fmt.Errorf("invalid container port %d", p.ContainerPort)
			}
			// ECS defaults an omitted protocol to tcp.
			protocol := p.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			if protocol != "tcp" && protocol != "udp" {
				return fmt.Errorf("unsupported port protocol %q", p.Protocol)
			}
			properties["port_"+strconv.Itoa(j)] = strconv.Itoa(p.ContainerPort) + "/" + protocol
		}
		for j, v := range c.Environment {
			if v.Name == "" {
				return errors.New("empty environment variable name")
			}
			properties["env_name_"+strconv.Itoa(j)] = v.Name
		}
		for j, v := range c.Secrets {
			if v.Name == "" {
				return errors.New("empty secret variable name")
			}
			properties["secret_name_"+strconv.Itoa(j)] = v.Name
		}
		containerID := resourceID("container", e.Region, t.TaskDefinitionArn+"/"+c.Name)
		out.Resources = append(out.Resources, domain.Resource{ID: containerID, Kind: "compute.container", Provider: "aws", Name: c.Name, Properties: properties, Capabilities: []string{"container_image"}, Sources: []string{source(e)}, Status: "DISCOVERED"})
		out.Dependencies = append(out.Dependencies, domain.Dependency{From: id, To: containerID, Relation: "contains", Status: "CONFIRMED", Evidence: source(e) + " taskDefinition.containerDefinitions.name=" + c.Name})
	}
	return nil
}

func parseServices(file *os.File, e entry, out *domain.Infrastructure) error {
	var response struct {
		Services []struct {
			ServiceArn     string `json:"serviceArn"`
			ServiceName    string `json:"serviceName"`
			TaskDefinition string `json:"taskDefinition"`
		} `json:"services"`
	}
	if err := readJSON(file, &response, false); err != nil {
		return err
	}
	if len(response.Services) == 0 {
		return errors.New("services must not be empty")
	}
	found := false
	for _, s := range response.Services {
		if s.ServiceArn == "" || s.ServiceName == "" || s.TaskDefinition == "" {
			return errors.New("serviceArn, serviceName and taskDefinition are required")
		}
		if s.ServiceArn != e.Resource {
			continue
		}
		found = true
		id := resourceID("service", e.Region, s.ServiceArn)
		out.Resources = append(out.Resources, domain.Resource{ID: id, Kind: "compute.service", Provider: "aws", Name: s.ServiceName, Sources: []string{source(e)}, Status: "DISCOVERED"})
		out.Dependencies = append(out.Dependencies, domain.Dependency{From: id, To: resourceID("task", e.Region, s.TaskDefinition), Relation: "runs", Status: "CONFIRMED", Evidence: source(e) + " services.taskDefinition=" + s.TaskDefinition})
	}
	if !found {
		return errors.New("manifest resource does not match any serviceArn")
	}
	return nil
}

func parseRDS(file *os.File, e entry, out *domain.Infrastructure) error {
	var response struct {
		DBInstances []struct {
			DBInstanceArn        string `json:"DBInstanceArn"`
			DBInstanceIdentifier string `json:"DBInstanceIdentifier"`
			Engine               string `json:"Engine"`
			EngineVersion        string `json:"EngineVersion"`
			Endpoint             struct {
				Address string `json:"Address"`
				Port    int    `json:"Port"`
			} `json:"Endpoint"`
		} `json:"DBInstances"`
	}
	if err := readJSON(file, &response, false); err != nil {
		return err
	}
	if len(response.DBInstances) == 0 {
		return errors.New("DBInstances must not be empty")
	}
	found := false
	for _, db := range response.DBInstances {
		if db.DBInstanceArn != e.Resource {
			continue
		}
		found = true
		if db.DBInstanceIdentifier == "" || db.Engine == "" {
			return errors.New("DBInstanceIdentifier and Engine are required")
		}
		if db.Engine != "postgres" {
			out.Findings = append(out.Findings, domain.Finding{Code: "UNSUPPORTED_ENGINE", Resource: resourceID("database", e.Region, db.DBInstanceArn), Message: "Database engine is not PostgreSQL: " + db.Engine})
		}
		props := map[string]string{"engine": db.Engine, "engine_version": db.EngineVersion}
		missing := []string{"database contents", "credentials"}
		if db.Endpoint.Address != "" {
			props["endpoint"] = db.Endpoint.Address
			props["port"] = strconv.Itoa(db.Endpoint.Port)
		} else {
			missing = append(missing, "endpoint")
		}
		out.Resources = append(out.Resources, domain.Resource{ID: resourceID("database", e.Region, db.DBInstanceArn), Kind: "database", Provider: "aws", Name: db.DBInstanceIdentifier, Properties: props, Capabilities: []string{"persistent_database"}, Sources: []string{source(e)}, Missing: missing, Status: "PARTIAL"})
	}
	if !found {
		return errors.New("manifest resource does not match any DBInstanceArn")
	}
	return nil
}

func bucket(out *domain.Infrastructure, e entry) *domain.Resource {
	id := resourceID("bucket", e.Region, e.Resource)
	for i := range out.Resources {
		if out.Resources[i].ID == id {
			out.Resources[i].Sources = append(out.Resources[i].Sources, source(e))
			return &out.Resources[i]
		}
	}
	out.Resources = append(out.Resources, domain.Resource{ID: id, Kind: "object_storage", Provider: "aws", Name: e.Resource, Properties: map[string]string{}, Missing: []string{"object contents", "bucket policy", "IAM permissions"}, Status: "PARTIAL", Sources: []string{source(e)}})
	return &out.Resources[len(out.Resources)-1]
}

func parseVersioning(file *os.File, e entry, out *domain.Infrastructure) error {
	var response struct {
		Status    string `json:"Status"`
		MFADelete string `json:"MFADelete"`
	}
	if err := readJSON(file, &response, false); err != nil {
		return err
	}
	if response.Status != "" && response.Status != "Enabled" && response.Status != "Suspended" {
		return fmt.Errorf("unexpected versioning status %q", response.Status)
	}
	r := bucket(out, e)
	r.Properties["versioning"] = response.Status
	r.Properties["mfa_delete"] = response.MFADelete
	if response.Status == "" {
		out.Findings = append(out.Findings, domain.Finding{Code: "VERSIONING_UNKNOWN", Resource: r.ID, Message: "Versioning status was absent"})
	}
	return nil
}

func parseEncryption(file *os.File, e entry, out *domain.Infrastructure) error {
	var response struct {
		ServerSideEncryptionConfiguration struct {
			Rules []struct {
				ApplyServerSideEncryptionByDefault struct {
					SSEAlgorithm string `json:"SSEAlgorithm"`
				} `json:"ApplyServerSideEncryptionByDefault"`
			} `json:"Rules"`
		} `json:"ServerSideEncryptionConfiguration"`
	}
	if err := readJSON(file, &response, false); err != nil {
		return err
	}
	if len(response.ServerSideEncryptionConfiguration.Rules) == 0 {
		return errors.New("encryption rules must not be empty")
	}
	r := bucket(out, e)
	for i, rule := range response.ServerSideEncryptionConfiguration.Rules {
		alg := rule.ApplyServerSideEncryptionByDefault.SSEAlgorithm
		if alg == "" {
			return errors.New("SSEAlgorithm is required")
		}
		r.Properties["encryption_"+strconv.Itoa(i)] = alg
		if strings.HasPrefix(alg, "aws:kms") {
			out.Findings = append(out.Findings, domain.Finding{Code: "KMS_MAPPING_REQUIRED", Resource: r.ID, Message: "KMS encryption cannot be mapped automatically"})
		}
	}
	return nil
}
