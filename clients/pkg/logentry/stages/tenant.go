package stages

import (
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"

	"github.com/grafana/loki/clients/pkg/promtail/client"
)

const (
	ErrTenantStageEmptyLabelSourceOrValue        = "label, source or value config are required"
	ErrTenantStageConflictingLabelSourceAndValue = "label, source and value are mutually exclusive: you should set source, value or label but not all"
)

type tenantStage struct {
	cfg    TenantConfig
	logger log.Logger
}

type TenantConfig struct {
	Label  string `mapstructure:"label"`
	Source string `mapstructure:"source"`
	Value  string `mapstructure:"value"`
}

// validateTenantConfig validates the tenant stage configuration
func validateTenantConfig(c TenantConfig) error {
	if c.Source == "" && c.Value == "" && c.Label == "" {
		return errors.New(ErrTenantStageEmptyLabelSourceOrValue)
	}

	if c.Source != "" && c.Value != "" || c.Label != "" && c.Value != "" || c.Source != "" && c.Label != "" {
		return errors.New(ErrTenantStageConflictingLabelSourceAndValue)
	}

	return nil
}

// newTenantStage creates a new tenant stage to override the tenant ID from extracted data
func newTenantStage(logger log.Logger, configs interface{}) (Stage, error) {
	cfg := TenantConfig{}
	err := mapstructure.Decode(configs, &cfg)
	if err != nil {
		return nil, err
	}

	err = validateTenantConfig(cfg)
	if err != nil {
		return nil, err
	}

	return toStage(&tenantStage{
		cfg:    cfg,
		logger: logger,
	}), nil
}

// Process implements Stage
func (s *tenantStage) Process(labels model.LabelSet, extracted map[string]interface{}, _ *time.Time, _ *string) {
	var tenantID string

	// Get tenant ID from source or configured value
	if s.cfg.Source != "" {
		tenantID = s.getTenantFromSourceField(extracted)
	} else if s.cfg.Label != "" {
		tenantID = s.getTenantFromLabel(labels)
	} else {
		tenantID = s.cfg.Value
	}

	// Skip an empty tenant ID (ie. failed to get the tenant from the source)
	if tenantID == "" {
		return
	}

	labels[client.ReservedLabelTenantID] = model.LabelValue(tenantID)
}

// Name implements Stage
func (s *tenantStage) Name() string {
	return StageTypeTenant
}

func (s *tenantStage) getTenantFromSourceField(extracted map[string]interface{}) string {
	// Get the tenant ID from the source data
	value, ok := extracted[s.cfg.Source]
	if !ok {
		if Debug {
			level.Debug(s.logger).Log("msg", "the tenant source does not exist in the extracted data", "source", s.cfg.Source)
		}
		return ""
	}

	// Convert the value to string
	tenantID, err := getString(value)
	if err != nil {
		if Debug {
			level.Debug(s.logger).Log("msg", "failed to convert value to string", "err", err, "type", reflect.TypeOf(value))
		}
		return ""
	}

	return tenantID
}

func (s *tenantStage) getTenantFromLabel(labels model.LabelSet) string {
	// Get the tenant ID from the label map
	tenantID, ok := labels[model.LabelName(s.cfg.Label)]

	if !ok {
		if Debug {
			level.Debug(s.logger).Log("msg", "the tenant source does not exist in the labels", "source", s.cfg.Source)
		}
		return ""
	}

	return string(tenantID)
}
