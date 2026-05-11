// Package normalizer converts integration-specific payloads to AlertEnvelope.
package normalizer

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/paladinai/paladinai/internal/alert"
	"github.com/paladinai/paladinai/services/paladin-ingest/internal/sanitizer"
)

// CloudWatchSNSNotification is the outer SNS envelope CloudWatch alarms arrive in.
type CloudWatchSNSNotification struct {
	Type      string    `json:"Type"`
	MessageID string    `json:"MessageId"`
	TopicARN  string    `json:"TopicArn"`
	Subject   string    `json:"Subject"`
	Message   string    `json:"Message"`
	Timestamp time.Time `json:"Timestamp"`
}

// CloudWatchAlarm is the inner alarm payload after parsing Message.
type CloudWatchAlarm struct {
	AlarmName        string            `json:"AlarmName"`
	AlarmDescription string            `json:"AlarmDescription"`
	AWSAccountID     string            `json:"AWSAccountId"`
	NewStateValue    string            `json:"NewStateValue"`
	NewStateReason   string            `json:"NewStateReason"`
	StateChangeTime  string            `json:"StateChangeTime"`
	Region           string            `json:"Region"`
	AlarmARN         string            `json:"AlarmArn"`
	OldStateValue    string            `json:"OldStateValue"`
	Trigger          CloudWatchTrigger `json:"Trigger"`
}

// CloudWatchTrigger carries the metric + dimensions that fired the alarm.
type CloudWatchTrigger struct {
	MetricName         string                `json:"MetricName"`
	Namespace          string                `json:"Namespace"`
	StatisticType      string                `json:"StatisticType"`
	Statistic          string                `json:"Statistic"`
	Period             int                   `json:"Period"`
	Threshold          float64               `json:"Threshold"`
	ComparisonOperator string                `json:"ComparisonOperator"`
	Dimensions         []CloudWatchDimension `json:"Dimensions"`
}

// CloudWatchDimension is one name/value pair attached to a metric.
type CloudWatchDimension struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

const cloudWatchTimeLayout = "2006-01-02T15:04:05.000-0700"

// NormalizeCloudWatch converts a CloudWatch SNS-wrapped alarm notification
// into AlertEnvelopes.
func NormalizeCloudWatch(tenantID string, raw json.RawMessage, log *zap.Logger) ([]alert.AlertEnvelope, error) {
	if log == nil {
		log = zap.NewNop()
	}
	var sns CloudWatchSNSNotification
	if err := json.Unmarshal(raw, &sns); err != nil {
		return nil, fmt.Errorf("unmarshal cloudwatch sns envelope: %w", err)
	}
	if sns.Message == "" {
		return nil, fmt.Errorf("cloudwatch sns message field is empty")
	}

	var ala CloudWatchAlarm
	if err := json.Unmarshal([]byte(sns.Message), &ala); err != nil {
		return nil, fmt.Errorf("unmarshal cloudwatch alarm message: %w", err)
	}

	rawLabels := map[string]string{}
	for _, d := range ala.Trigger.Dimensions {
		if d.Name == "" {
			continue
		}
		rawLabels[d.Name] = d.Value
	}
	if ala.Trigger.Namespace != "" {
		rawLabels["namespace"] = ala.Trigger.Namespace
	}
	if ala.Trigger.MetricName != "" {
		rawLabels["metric_name"] = ala.Trigger.MetricName
	}
	if ala.Region != "" {
		rawLabels["region"] = ala.Region
	}
	if ala.AlarmName != "" {
		rawLabels["alertname"] = ala.AlarmName
	}
	if ala.AWSAccountID != "" {
		rawLabels["aws_account_id"] = ala.AWSAccountID
	}

	description := ala.AlarmDescription
	if description == "" {
		description = ala.NewStateReason
	}

	rawAnnotations := map[string]string{}
	if description != "" {
		rawAnnotations["description"] = description
	}

	labelResult := sanitizer.SanitizeMap(rawLabels)
	annotationResult := sanitizer.SanitizeMap(rawAnnotations)
	labels := labelResult.Values
	annotations := annotationResult.Values

	if len(labelResult.ChangedKeys) > 0 || len(labelResult.DroppedKeys) > 0 {
		log.Warn("prompt injection sanitized in alert labels",
			zap.String("tenant_id", tenantID),
			zap.Strings("changed_keys", labelResult.ChangedKeys),
			zap.Strings("dropped_keys", labelResult.DroppedKeys),
		)
	}
	if len(annotationResult.ChangedKeys) > 0 || len(annotationResult.DroppedKeys) > 0 {
		log.Warn("prompt injection sanitized in alert annotations",
			zap.String("tenant_id", tenantID),
			zap.Strings("changed_keys", annotationResult.ChangedKeys),
			zap.Strings("dropped_keys", annotationResult.DroppedKeys),
		)
	}

	status := mapCloudWatchStatus(ala.NewStateValue)
	severity := mapCloudWatchSeverity(ala.NewStateValue)
	fp := alert.ComputeFingerprint(alert.SourceCloudWatch, labels)

	startsAt := parseCloudWatchTime(ala.StateChangeTime)
	if startsAt.IsZero() {
		if !sns.Timestamp.IsZero() {
			startsAt = sns.Timestamp.UTC()
		} else {
			startsAt = time.Now().UTC()
		}
	}

	env := alert.AlertEnvelope{
		ID:          uuid.New().String(),
		Fingerprint: fp,
		TenantID:    tenantID,
		Severity:    severity,
		Status:      status,
		Source:      alert.SourceCloudWatch,
		Title:       ala.AlarmName,
		Description: annotations["description"],
		Labels:      labels,
		Annotations: annotations,
		StartsAt:    startsAt,
		ReceivedAt:  time.Now().UTC(),
		AgentState:  "pending",
		Payload:     append(json.RawMessage(nil), raw...),
	}

	if status == alert.StatusResolved {
		t := time.Now().UTC()
		env.EndsAt = &t
	}

	return []alert.AlertEnvelope{env}, nil
}

func parseCloudWatchTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(cloudWatchTimeLayout, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func mapCloudWatchStatus(state string) alert.Status {
	switch strings.ToUpper(state) {
	case "ALARM", "INSUFFICIENT_DATA":
		return alert.StatusFiring
	case "OK":
		return alert.StatusResolved
	default:
		return alert.StatusFiring
	}
}

func mapCloudWatchSeverity(state string) alert.Severity {
	switch strings.ToUpper(state) {
	case "ALARM":
		return alert.SeverityP2
	case "INSUFFICIENT_DATA":
		return alert.SeverityP3
	case "OK":
		return alert.SeverityP4
	default:
		return alert.SeverityP3
	}
}
