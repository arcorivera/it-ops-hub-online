package tickets

import (
	"encoding/csv"
	"io"
	"net/http"
	"strings"

	"itopshub/backend/internal/response"
)

// ImportRowError describes why a specific CSV row was skipped or failed
// (spec section 53: "Show error details").
type ImportRowError struct {
	Row     int    `json:"row"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type ImportResult struct {
	Imported int              `json:"imported"`
	Skipped  int              `json:"skipped"`
	Failed   []ImportRowError `json:"failed"`
}

// expectedHeaders defines the accepted CSV column names (case-insensitive,
// order-independent). Only "title" is required; everything else falls back
// to sane defaults matching ticket creation via the API.
var expectedHeaders = []string{"title", "description", "severity", "priority", "environment"}

// ImportCSV parses an uploaded CSV file and creates one ticket per valid
// row, using the exact same validation and SLA-assignment path as normal
// ticket creation (Create), so imported tickets are indistinguishable from
// manually created ones.
func (h *Handler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB cap
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		response.BadRequest(w, "File too large or invalid upload (max 10MB)", nil)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		response.BadRequest(w, "No file provided", nil)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow rows with fewer columns than the header (missing trailing optional fields)

	header, err := reader.Read()
	if err != nil {
		response.BadRequest(w, "CSV file is empty or unreadable", nil)
		return
	}

	colIndex := map[string]int{}
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := colIndex["title"]; !ok {
		response.BadRequest(w, "CSV must include a 'title' column", map[string]string{
			"expectedColumns": strings.Join(expectedHeaders, ", "),
		})
		return
	}

	result := ImportResult{}
	actorID := h.actor(r)
	rowNum := 1 // header was row 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err != nil {
			result.Failed = append(result.Failed, ImportRowError{Row: rowNum, Message: "Malformed CSV row: " + err.Error()})
			continue
		}

		title := getCol(record, colIndex, "title")
		if strings.TrimSpace(title) == "" {
			result.Skipped++
			result.Failed = append(result.Failed, ImportRowError{Row: rowNum, Message: "Title is required"})
			continue
		}

		severity := strings.ToUpper(strings.TrimSpace(getCol(record, colIndex, "severity")))
		if severity == "" {
			severity = SeverityS3
		} else if !ValidSeverities[severity] {
			result.Failed = append(result.Failed, ImportRowError{Row: rowNum, Title: title, Message: "Invalid severity: " + severity})
			continue
		}

		priority := strings.TrimSpace(getCol(record, colIndex, "priority"))
		if priority == "" {
			priority = PriorityMedium
		} else if !ValidPriorities[priority] {
			result.Failed = append(result.Failed, ImportRowError{Row: rowNum, Title: title, Message: "Invalid priority: " + priority})
			continue
		}

		environment := strings.ToUpper(strings.TrimSpace(getCol(record, colIndex, "environment")))
		if environment == "" {
			environment = EnvProduction
		} else if !ValidEnvironments[environment] {
			result.Failed = append(result.Failed, ImportRowError{Row: rowNum, Title: title, Message: "Invalid environment: " + environment})
			continue
		}

		description := getCol(record, colIndex, "description")

		_, err = h.repo.Create(CreateInput{
			Title: title, Description: description, Severity: severity, Priority: priority, Environment: environment,
			RequesterID: &actorID,
		}, actorID)
		if err != nil {
			result.Failed = append(result.Failed, ImportRowError{Row: rowNum, Title: title, Message: "Database error: " + err.Error()})
			continue
		}
		result.Imported++
	}

	h.audit.Log(actorID, "IMPORT", "ticket", "", map[string]interface{}{
		"imported": result.Imported, "skipped": result.Skipped, "failed": len(result.Failed),
	}, r.RemoteAddr)

	response.JSON(w, http.StatusOK, result)
}

func getCol(record []string, colIndex map[string]int, name string) string {
	i, ok := colIndex[name]
	if !ok || i >= len(record) {
		return ""
	}
	return record[i]
}
