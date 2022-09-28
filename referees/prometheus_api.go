// The only purpose for having this files and interfaces redefined
// in it is to make automatic mocks generator (`make mocks`) able to
// create mocks of some Prometheus interfaces - which are not present
// in the original packages but are required to make our tests simpler
// and more "unit".

package referees

import (
	prometheusV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

//nolint:unused // see file header
//go:generate mockery --name=prometheusAPI --inpackage
type prometheusAPI interface {
	prometheusV1.API
}

//nolint:unused // see file header
//go:generate mockery --name=prometheusValue --inpackage
type prometheusValue interface {
	model.Value
}
