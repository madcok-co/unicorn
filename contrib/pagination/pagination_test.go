package pagination

import (
	"testing"
)

func TestDefaultOffsetParams(t *testing.T) {
	params := DefaultOffsetParams()

	if params.Page != 1 {
		t.Errorf("expected default page to be 1, got %d", params.Page)
	}

	if params.Limit != 20 {
		t.Errorf("expected default limit to be 20, got %d", params.Limit)
	}

	if params.MaxLimit != 100 {
		t.Errorf("expected default max limit to be 100, got %d", params.MaxLimit)
	}

	if params.Order != "asc" {
		t.Errorf("expected default order to be asc, got %s", params.Order)
	}
}

func TestParseOffsetParams(t *testing.T) {
	tests := []struct {
		name          string
		page          int
		limit         int
		sort          string
		order         string
		expectedPage  int
		expectedLimit int
	}{
		{"default values", 0, 0, "", "", 1, 20},
		{"custom values", 2, 50, "name", "desc", 2, 50},
		{"limit exceeds max", 1, 200, "", "", 1, 100},
		{"invalid page", -1, 10, "", "", 1, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := ParseOffsetParams(tt.page, tt.limit, tt.sort, tt.order)

			if params.Page != tt.expectedPage {
				t.Errorf("expected page %d, got %d", tt.expectedPage, params.Page)
			}

			if params.Limit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, params.Limit)
			}
		})
	}
}

func TestOffsetParams_Offset(t *testing.T) {
	tests := []struct {
		page           int
		limit          int
		expectedOffset int
	}{
		{1, 20, 0},
		{2, 20, 20},
		{3, 20, 40},
		{1, 50, 0},
		{2, 50, 50},
	}

	for _, tt := range tests {
		params := &OffsetParams{
			Page:  tt.page,
			Limit: tt.limit,
		}

		offset := params.Offset()
		if offset != tt.expectedOffset {
			t.Errorf("page=%d, limit=%d: expected offset %d, got %d",
				tt.page, tt.limit, tt.expectedOffset, offset)
		}
	}
}

func TestOffsetParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  *OffsetParams
		wantErr bool
	}{
		{
			name: "valid params",
			params: &OffsetParams{
				Page:     1,
				Limit:    20,
				Order:    "asc",
				MaxLimit: 100,
			},
			wantErr: false,
		},
		{
			name: "invalid page",
			params: &OffsetParams{
				Page:     0,
				Limit:    20,
				Order:    "asc",
				MaxLimit: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid limit",
			params: &OffsetParams{
				Page:     1,
				Limit:    0,
				Order:    "asc",
				MaxLimit: 100,
			},
			wantErr: true,
		},
		{
			name: "limit exceeds max",
			params: &OffsetParams{
				Page:     1,
				Limit:    200,
				Order:    "asc",
				MaxLimit: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid order",
			params: &OffsetParams{
				Page:     1,
				Limit:    20,
				Order:    "invalid",
				MaxLimit: 100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncodeDecodeCursor(t *testing.T) {
	tests := []string{
		"123",
		"user-abc-123",
		"2024-01-01T00:00:00Z",
	}

	for _, original := range tests {
		encoded := EncodeCursor(original)
		if encoded == original {
			t.Error("expected cursor to be encoded")
		}

		decoded, err := DecodeCursor(encoded)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if decoded != original {
			t.Errorf("expected %s, got %s", original, decoded)
		}
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	_, err := DecodeCursor("invalid-cursor!!!")
	if err == nil {
		t.Error("expected error for invalid cursor")
	}
}

func TestNewOffsetResult(t *testing.T) {
	data := []string{"item1", "item2", "item3"}
	total := int64(100)
	params := &OffsetParams{
		Page:  2,
		Limit: 20,
	}

	result := NewOffsetResult(data, total, params)

	if result.Total != 100 {
		t.Errorf("expected total 100, got %d", result.Total)
	}

	if result.Page != 2 {
		t.Errorf("expected page 2, got %d", result.Page)
	}

	if result.Limit != 20 {
		t.Errorf("expected limit 20, got %d", result.Limit)
	}

	if result.TotalPages != 5 {
		t.Errorf("expected 5 pages (100/20), got %d", result.TotalPages)
	}

	if !result.HasPrevious {
		t.Error("expected HasPrevious to be true on page 2")
	}

	if !result.HasNext {
		t.Error("expected HasNext to be true on page 2 of 5")
	}
}

func TestNewOffsetResult_FirstPage(t *testing.T) {
	params := &OffsetParams{Page: 1, Limit: 20}
	result := NewOffsetResult([]string{}, 100, params)

	if result.HasPrevious {
		t.Error("expected HasPrevious to be false on first page")
	}

	if !result.HasNext {
		t.Error("expected HasNext to be true")
	}
}

func TestNewOffsetResult_LastPage(t *testing.T) {
	params := &OffsetParams{Page: 5, Limit: 20}
	result := NewOffsetResult([]string{}, 100, params)

	if !result.HasPrevious {
		t.Error("expected HasPrevious to be true on last page")
	}

	if result.HasNext {
		t.Error("expected HasNext to be false on last page")
	}
}

func TestNewCursorResult(t *testing.T) {
	data := []string{"item1", "item2"}
	nextCursor := EncodeCursor("next-123")
	prevCursor := EncodeCursor("prev-100")

	result := NewCursorResult(data, nextCursor, prevCursor, true, true)

	if result.NextCursor != nextCursor {
		t.Errorf("expected next cursor %s, got %s", nextCursor, result.NextCursor)
	}

	if result.PrevCursor != prevCursor {
		t.Errorf("expected prev cursor %s, got %s", prevCursor, result.PrevCursor)
	}

	if !result.HasNext {
		t.Error("expected HasNext to be true")
	}

	if !result.HasPrev {
		t.Error("expected HasPrev to be true")
	}
}

func TestBuildOrderByClause(t *testing.T) {
	tests := []struct {
		sort     string
		order    string
		expected string
	}{
		{"name", "asc", "name ASC"},
		{"created_at", "desc", "created_at DESC"},
		{"user.email", "asc", "user.email ASC"},
		{"", "asc", ""},
		{"name", "", "name ASC"},
	}

	for _, tt := range tests {
		result := BuildOrderByClause(tt.sort, tt.order)
		if result != tt.expected {
			t.Errorf("BuildOrderByClause(%q, %q) = %q, want %q",
				tt.sort, tt.order, result, tt.expected)
		}
	}
}

func TestSanitizeField(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"name", "name"},
		{"user_id", "user_id"},
		{"user.email", "user.email"},
		{"DROP TABLE", "DROPTABLE"},
		{"name; DROP TABLE", "nameDROPTABLE"},
		{"user_id'--", "user_id"},
	}

	for _, tt := range tests {
		result := sanitizeField(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeField(%q) = %q, want %q",
				tt.input, result, tt.expected)
		}
	}
}

func TestBuildLinks(t *testing.T) {
	baseURL := "https://api.example.com/users"
	page := 2
	totalPages := 5

	links := BuildLinks(baseURL, page, totalPages)

	expectedSelf := "https://api.example.com/users?page=2"
	if links.Self != expectedSelf {
		t.Errorf("expected self %s, got %s", expectedSelf, links.Self)
	}

	expectedFirst := "https://api.example.com/users?page=1"
	if links.First != expectedFirst {
		t.Errorf("expected first %s, got %s", expectedFirst, links.First)
	}

	expectedPrev := "https://api.example.com/users?page=1"
	if links.Prev != expectedPrev {
		t.Errorf("expected prev %s, got %s", expectedPrev, links.Prev)
	}

	expectedNext := "https://api.example.com/users?page=3"
	if links.Next != expectedNext {
		t.Errorf("expected next %s, got %s", expectedNext, links.Next)
	}

	expectedLast := "https://api.example.com/users?page=5"
	if links.Last != expectedLast {
		t.Errorf("expected last %s, got %s", expectedLast, links.Last)
	}
}

func TestBuildLinks_FirstPage(t *testing.T) {
	links := BuildLinks("https://api.example.com/users", 1, 5)

	if links.Prev != "" {
		t.Errorf("expected no prev link on first page, got %s", links.Prev)
	}

	if links.Next == "" {
		t.Error("expected next link on first page")
	}
}

func TestBuildLinks_LastPage(t *testing.T) {
	links := BuildLinks("https://api.example.com/users", 5, 5)

	if links.Prev == "" {
		t.Error("expected prev link on last page")
	}

	if links.Next != "" {
		t.Errorf("expected no next link on last page, got %s", links.Next)
	}
}

func TestBuildCursorLinks(t *testing.T) {
	baseURL := "https://api.example.com/users"
	nextCursor := "next123"
	prevCursor := "prev100"

	links := BuildCursorLinks(baseURL, nextCursor, prevCursor)

	if links.Self != baseURL {
		t.Errorf("expected self %s, got %s", baseURL, links.Self)
	}

	expectedNext := "https://api.example.com/users?cursor=next123"
	if links.Next != expectedNext {
		t.Errorf("expected next %s, got %s", expectedNext, links.Next)
	}

	expectedPrev := "https://api.example.com/users?cursor=prev100"
	if links.Prev != expectedPrev {
		t.Errorf("expected prev %s, got %s", expectedPrev, links.Prev)
	}
}

func TestParsePageFromQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},
		{"1", 1},
		{"5", 5},
		{"0", 1},
		{"-1", 1},
		{"abc", 1},
		{"10", 10},
	}

	for _, tt := range tests {
		result := ParsePageFromQuery(tt.input)
		if result != tt.expected {
			t.Errorf("ParsePageFromQuery(%q) = %d, want %d",
				tt.input, result, tt.expected)
		}
	}
}

func TestParseLimitFromQuery(t *testing.T) {
	tests := []struct {
		input        string
		defaultLimit int
		maxLimit     int
		expected     int
	}{
		{"", 20, 100, 20},
		{"50", 20, 100, 50},
		{"200", 20, 100, 100},
		{"0", 20, 100, 20},
		{"-1", 20, 100, 20},
		{"abc", 20, 100, 20},
		{"10", 20, 100, 10},
	}

	for _, tt := range tests {
		result := ParseLimitFromQuery(tt.input, tt.defaultLimit, tt.maxLimit)
		if result != tt.expected {
			t.Errorf("ParseLimitFromQuery(%q, %d, %d) = %d, want %d",
				tt.input, tt.defaultLimit, tt.maxLimit, result, tt.expected)
		}
	}
}

func TestDefaultCursorEncoder(t *testing.T) {
	encoder := &DefaultCursorEncoder{}

	value := "user-123"
	encoded := encoder.Encode(value)

	if encoded == value {
		t.Error("expected value to be encoded")
	}

	decoded, err := encoder.Decode(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded != value {
		t.Errorf("expected %s, got %v", value, decoded)
	}
}

func TestDefaultCursorParams(t *testing.T) {
	params := DefaultCursorParams()

	if params.Limit != 20 {
		t.Errorf("expected default limit to be 20, got %d", params.Limit)
	}

	if params.Order != "asc" {
		t.Errorf("expected default order to be asc, got %s", params.Order)
	}

	if params.MaxLimit != 100 {
		t.Errorf("expected default max limit to be 100, got %d", params.MaxLimit)
	}
}

func TestParseCursorParams(t *testing.T) {
	params := ParseCursorParams("cursor123", 50, "created_at", "desc")

	if params.Cursor != "cursor123" {
		t.Errorf("expected cursor cursor123, got %s", params.Cursor)
	}

	if params.Limit != 50 {
		t.Errorf("expected limit 50, got %d", params.Limit)
	}

	if params.Sort != "created_at" {
		t.Errorf("expected sort created_at, got %s", params.Sort)
	}

	if params.Order != "desc" {
		t.Errorf("expected order desc, got %s", params.Order)
	}
}

func TestCursorParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  *CursorParams
		wantErr bool
	}{
		{
			name: "valid params",
			params: &CursorParams{
				Limit:    20,
				Order:    "asc",
				MaxLimit: 100,
			},
			wantErr: false,
		},
		{
			name: "invalid limit",
			params: &CursorParams{
				Limit:    0,
				Order:    "asc",
				MaxLimit: 100,
			},
			wantErr: true,
		},
		{
			name: "limit exceeds max",
			params: &CursorParams{
				Limit:    200,
				Order:    "asc",
				MaxLimit: 100,
			},
			wantErr: true,
		},
		{
			name: "invalid order",
			params: &CursorParams{
				Limit:    20,
				Order:    "random",
				MaxLimit: 100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
