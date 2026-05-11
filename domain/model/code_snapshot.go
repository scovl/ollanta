package model

// CodeSnapshotFile is the read-only source payload stored for a scanned file.
type CodeSnapshotFile struct {
	Path          string `json:"path"`
	Language      string `json:"language"`
	Content       string `json:"content,omitempty"`
	SizeBytes     int    `json:"size_bytes,omitempty"`
	LineCount     int    `json:"line_count,omitempty"`
	IsTruncated   bool   `json:"is_truncated,omitempty"`
	IsOmitted     bool   `json:"is_omitted,omitempty"`
	OmittedReason string `json:"omitted_reason,omitempty"`
}

// CodeSnapshot holds the latest uploaded source snapshot for a scope.
type CodeSnapshot struct {
	Files          []CodeSnapshotFile `json:"files,omitempty"`
	TotalFiles     int                `json:"total_files,omitempty"`
	StoredFiles    int                `json:"stored_files,omitempty"`
	TruncatedFiles int                `json:"truncated_files,omitempty"`
	OmittedFiles   int                `json:"omitted_files,omitempty"`
	StoredBytes    int                `json:"stored_bytes,omitempty"`
	MaxFileBytes   int                `json:"max_file_bytes,omitempty"`
	MaxTotalBytes  int                `json:"max_total_bytes,omitempty"`
}

// CodeSnapshotState binds a snapshot payload to the project, scan, and scope it belongs to.
type CodeSnapshotState struct {
	ProjectID int64         `json:"project_id"`
	ScanID    int64         `json:"scan_id"`
	Scope     AnalysisScope `json:"scope"`
	Snapshot  CodeSnapshot  `json:"snapshot"`
}
