package model

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
)

type TagStatus string

const (
	TagStatusActive     TagStatus = "active"
	TagStatusDiscovered TagStatus = "discovered"
	TagStatusDeprecated TagStatus = "deprecated"
)

type TagSource string

const (
	TagSourceManual     TagSource = "manual"
	TagSourceScan       TagSource = "scan"
	TagSourceBackfill   TagSource = "backfill"
	TagSourceAssignment TagSource = "assignment"
)

type TagOwnerType string

const (
	TagOwnerNone  TagOwnerType = ""
	TagOwnerUser  TagOwnerType = "user"
	TagOwnerGroup TagOwnerType = "group"
	TagOwnerTeam  TagOwnerType = "team"
)

type TagTargetType string

const (
	TagTargetIssue      TagTargetType = "issue"
	TagTargetProject    TagTargetType = "project"
	TagTargetRule       TagTargetType = "rule"
	TagTargetCustomRule TagTargetType = "custom_rule"
)

type SavedFilterVisibility string

const (
	SavedFilterPrivate SavedFilterVisibility = "private"
	SavedFilterShared  SavedFilterVisibility = "shared"
)

var (
	ErrInvalidTagKey   = errors.New("invalid tag key")
	ErrInvalidTagColor = errors.New("invalid tag color")

	tagKeyPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{0,62}$`)
	tagColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	reservedTagKeys = map[string]bool{
		"all":       true,
		"none":      true,
		"null":      true,
		"undefined": true,
	}
)

type TagCatalogEntry struct {
	ID             int64           `json:"id"`
	Key            string          `json:"key"`
	DisplayName    string          `json:"display_name"`
	Description    string          `json:"description"`
	Color          string          `json:"color"`
	OwnerType      TagOwnerType    `json:"owner_type,omitempty"`
	OwnerID        int64           `json:"owner_id,omitempty"`
	OwnerName      string          `json:"owner_name,omitempty"`
	Scope          string          `json:"scope"`
	Status         TagStatus       `json:"status"`
	Source         TagSource       `json:"source"`
	ReplacementKey string          `json:"replacement_key,omitempty"`
	Aliases        []TagAlias      `json:"aliases,omitempty"`
	Usage          TagUsageSummary `json:"usage"`
	CreatedBy      int64           `json:"created_by,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type TagAlias struct {
	ID        int64     `json:"id"`
	TagKey    string    `json:"tag_key"`
	Alias     string    `json:"alias"`
	CreatedAt time.Time `json:"created_at"`
}

type TagUsageSummary struct {
	IssueCount       int `json:"issue_count"`
	ProjectCount     int `json:"project_count"`
	RuleCount        int `json:"rule_count"`
	CustomRuleCount  int `json:"custom_rule_count"`
	SavedFilterCount int `json:"saved_filter_count"`
}

type TagAuditEntry struct {
	ID          int64          `json:"id"`
	TagKey      string         `json:"tag_key"`
	Action      string         `json:"action"`
	TargetType  TagTargetType  `json:"target_type,omitempty"`
	TargetID    int64          `json:"target_id,omitempty"`
	TargetKey   string         `json:"target_key,omitempty"`
	ActorUserID int64          `json:"actor_user_id,omitempty"`
	OldState    map[string]any `json:"old_state,omitempty"`
	NewState    map[string]any `json:"new_state,omitempty"`
	Summary     map[string]any `json:"summary,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type TagFilter struct {
	Query  string
	Status TagStatus
	Owner  string
	Scope  string
	Limit  int
	Offset int
}

type TagUpdate struct {
	DisplayName    *string
	Description    *string
	Color          *string
	OwnerType      *TagOwnerType
	OwnerID        *int64
	OwnerName      *string
	Scope          *string
	Status         *TagStatus
	ReplacementKey *string
	Aliases        []string
	ActorUserID    int64
}

type TagDetail struct {
	Tag          TagCatalogEntry `json:"tag"`
	Audit        []TagAuditEntry `json:"audit,omitempty"`
	RelatedTags  []string        `json:"related_tags,omitempty"`
	SavedFilters []SavedFilter   `json:"saved_filters,omitempty"`
}

type TagAssignment struct {
	ID          int64         `json:"id"`
	TargetType  TagTargetType `json:"target_type"`
	TargetID    int64         `json:"target_id,omitempty"`
	TargetKey   string        `json:"target_key,omitempty"`
	TagKey      string        `json:"tag_key"`
	Source      TagSource     `json:"source"`
	ActorUserID int64         `json:"actor_user_id,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
}

type TagBulkEditRequest struct {
	TargetType  TagTargetType `json:"target_type"`
	TargetIDs   []int64       `json:"target_ids,omitempty"`
	TargetKeys  []string      `json:"target_keys,omitempty"`
	AddTags     []string      `json:"add_tags,omitempty"`
	RemoveTags  []string      `json:"remove_tags,omitempty"`
	ActorUserID int64         `json:"actor_user_id,omitempty"`
	Reason      string        `json:"reason,omitempty"`
}

type TagBulkEditPreview struct {
	TargetType       TagTargetType        `json:"target_type"`
	TargetCount      int                  `json:"target_count"`
	AddTags          []string             `json:"add_tags"`
	RemoveTags       []string             `json:"remove_tags"`
	ValidationErrors []string             `json:"validation_errors,omitempty"`
	Skipped          []TagBulkEditSkipped `json:"skipped,omitempty"`
	Changes          []TagBulkEditChange  `json:"changes,omitempty"`
}

type TagBulkEditSkipped struct {
	TargetID  int64  `json:"target_id,omitempty"`
	TargetKey string `json:"target_key,omitempty"`
	Reason    string `json:"reason"`
}

type TagBulkEditChange struct {
	TargetID    int64    `json:"target_id,omitempty"`
	TargetKey   string   `json:"target_key,omitempty"`
	Before      []string `json:"before"`
	After       []string `json:"after"`
	AddedTags   []string `json:"added_tags,omitempty"`
	RemovedTags []string `json:"removed_tags,omitempty"`
}

type TagBulkEditResult struct {
	TagBulkEditPreview
	UpdatedCount int `json:"updated_count"`
	FailedCount  int `json:"failed_count"`
}

type SavedFilter struct {
	ID          int64                 `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	OwnerUserID int64                 `json:"owner_user_id,omitempty"`
	Visibility  SavedFilterVisibility `json:"visibility"`
	FilterType  string                `json:"filter_type"`
	Criteria    map[string]any        `json:"criteria"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
}

func NormalizeTagKey(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}
	parts := strings.Fields(trimmed)
	if len(parts) > 1 {
		trimmed = strings.Join(parts, "-")
	}
	trimmed = strings.Trim(trimmed, "-_")
	return trimmed
}

func NormalizeTagKeys(raw []string) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(raw))
	for _, value := range raw {
		key := NormalizeTagKey(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func ValidateTagKey(key string) error {
	normalized := NormalizeTagKey(key)
	if normalized == "" || reservedTagKeys[normalized] || !tagKeyPattern.MatchString(normalized) {
		return ErrInvalidTagKey
	}
	return nil
}

func ValidateTagColor(color string) error {
	if color == "" || tagColorPattern.MatchString(color) {
		return nil
	}
	return ErrInvalidTagColor
}

func CanApplyTag(status TagStatus) bool {
	return status != TagStatusDeprecated
}

func DefaultTagDisplayName(key string) string {
	key = NormalizeTagKey(key)
	if key == "" {
		return ""
	}
	parts := strings.FieldsFunc(key, func(value rune) bool {
		return value == '-' || value == '_' || value == ':' || value == '.'
	})
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
