package db

import (
	"context"
	"github.com/google/uuid"
	"sessionflow/apps/api/internal/usecase/longitudinal"
)

func (r *ClinicalLongitudinalRepository) ProjectExports(ctx context.Context, t, c uuid.UUID, offset int) (longitudinal.ProjectExportPage, error) {
	out := longitudinal.ProjectExportPage{Items: []longitudinal.ProjectExportSummary{}, Offset: offset}
	rows, err := r.pool.Query(ctx, `SELECT id,generated_at,state_version,content_hash,artifact_json->'case_runtime_profile'->>'process_ref' FROM clinical_project_exports WHERE tenant_id=$1 AND client_id=$2 ORDER BY generated_at DESC,id DESC LIMIT 26 OFFSET $3`, t, c, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var x longitudinal.ProjectExportSummary
		if err := rows.Scan(&x.ID, &x.GeneratedAt, &x.StateVersion, &x.ContentHash, &x.ProcessRef); err != nil {
			return out, err
		}
		out.Items = append(out.Items, x)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(out.Items) > 25 {
		out.HasMore = true
		out.Items = out.Items[:25]
	}
	return out, nil
}
