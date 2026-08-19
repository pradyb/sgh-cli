// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"
	"time"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/pkg/context"
)

// AuditListRequest contains parameters for listing audit log entries.
type AuditListRequest struct {
	OrgName string
	Phrase  string
	Include string
	Count   int
}

// ListAuditLog returns audit log entries for the given org.
func ListAuditLog(ctx *context.Context, req AuditListRequest) model.AuditLogResponse {
	entries, err := service.GetAuditLog(ctx, req.OrgName, req.Phrase, req.Include, req.Count)
	if err != nil {
		return model.AuditLogResponse{
			OrgName:      req.OrgName,
			ErrorMessage: fmt.Sprintf("failed to get audit log: %v", err),
		}
	}
	return model.AuditLogResponse{OrgName: req.OrgName, Entries: entries}
}

// FormatTimestamp converts the audit log millisecond epoch timestamp to a human-readable string.
func FormatTimestamp(ms int64) string {
	if ms == 0 {
		return ""
	}
	t := time.UnixMilli(ms).UTC()
	return t.Format("2006-01-02 15:04:05 UTC")
}
