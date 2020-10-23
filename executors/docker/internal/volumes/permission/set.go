package permission

import "context"

type Setter interface {
	Set(ctx context.Context, volumeName string, labels map[string]string) error
}
