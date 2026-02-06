package runtime

import (
	"context"

	"github.com/cilo/cilo/pkg/models"
)

type Provider interface {
	Up(ctx context.Context, env *models.Environment, opts UpOptions) error
	Down(ctx context.Context, env *models.Environment) error
	Destroy(ctx context.Context, env *models.Environment) error

	CreateNetwork(ctx context.Context, env *models.Environment) error
	RemoveNetwork(ctx context.Context, envName string) error

	GetContainerIP(ctx context.Context, envName, serviceName string) (string, error)
	GetContainerIPs(ctx context.Context, envName string, services []string) (map[string]string, error)
	GetServiceStatus(ctx context.Context, project, envName string) (map[string]string, error)

	Logs(ctx context.Context, project, envName, serviceName string, opts LogOptions) error
	Exec(ctx context.Context, project, envName, serviceName string, command []string, opts ExecOptions) error
	Compose(ctx context.Context, project, envName string, opts ComposeOptions) error
}
