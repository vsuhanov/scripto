package tui

import "testing"

func TestCalculateScrollWindow(t *testing.T) {
	tests := []struct {
		name           string
		selectedLine   int
		totalLines     int
		visibleHeight  int
		expectStart    int
		expectEnd      int
		expectLineSize int
	}{
		{
			name:           "selected in middle",
			selectedLine:   5,
			totalLines:     37,
			visibleHeight:  10,
			expectStart:    0,
			expectEnd:      10,
			expectLineSize: 10,
		},
		{
			name:           "selected near start",
			selectedLine:   2,
			totalLines:     37,
			visibleHeight:  10,
			expectStart:    0,
			expectEnd:      10,
			expectLineSize: 10,
		},
		{
			name:           "selected near end",
			selectedLine:   35,
			totalLines:     37,
			visibleHeight:  10,
			expectStart:    27,
			expectEnd:      37,
			expectLineSize: 10,
		},
		{
			name:           "selected at start",
			selectedLine:   0,
			totalLines:     37,
			visibleHeight:  10,
			expectStart:    0,
			expectEnd:      10,
			expectLineSize: 10,
		},
		{
			name:           "selected at end",
			selectedLine:   36,
			totalLines:     37,
			visibleHeight:  10,
			expectStart:    27,
			expectEnd:      37,
			expectLineSize: 10,
		},
		{
			name:           "total lines less than visible",
			selectedLine:   2,
			totalLines:     5,
			visibleHeight:  10,
			expectStart:    0,
			expectEnd:      5,
			expectLineSize: 5,
		},
		{
			name:           "total lines equal visible",
			selectedLine:   5,
			totalLines:     10,
			visibleHeight:  10,
			expectStart:    0,
			expectEnd:      10,
			expectLineSize: 10,
		},
		{
			name:           "small visible height",
			selectedLine:   10,
			totalLines:     20,
			visibleHeight:  3,
			expectStart:    8,
			expectEnd:      11,
			expectLineSize: 3,
		},
		{
			name:           "selected in middle of large list",
			selectedLine:   50,
			totalLines:     100,
			visibleHeight:  10,
			expectStart:    45,
			expectEnd:      55,
			expectLineSize: 10,
		},
		{
			name:           "odd visible height",
			selectedLine:   15,
			totalLines:     30,
			visibleHeight:  7,
			expectStart:    11,
			expectEnd:      18,
			expectLineSize: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := calculateScrollWindow(tt.selectedLine, tt.totalLines, tt.visibleHeight)

			actualSize := end - start

			if start != tt.expectStart {
				t.Errorf("expected start=%d, got %d", tt.expectStart, start)
			}
			if end != tt.expectEnd {
				t.Errorf("expected end=%d, got %d", tt.expectEnd, end)
			}
			if actualSize != tt.expectLineSize {
				t.Errorf("expected window size=%d, got %d", tt.expectLineSize, actualSize)
			}
			if actualSize > tt.visibleHeight {
				t.Errorf("window size %d exceeds visible height %d", actualSize, tt.visibleHeight)
			}
			if start < 0 {
				t.Errorf("start index %d is negative", start)
			}
			if end > tt.totalLines {
				t.Errorf("end index %d exceeds line count %d", end, tt.totalLines)
			}
		})
	}
}

func TestCalculateScrollWindowInvariants(t *testing.T) {
	tests := []struct {
		selectedLine  int
		totalLines    int
		visibleHeight int
	}{
		{0, 1, 1},
		{0, 1, 10},
		{0, 2, 1},
		{1, 2, 1},
		{50, 100, 1},
		{0, 100, 5},
		{99, 100, 5},
		{50, 1000, 20},
		{500, 1000, 20},
	}

	for _, tt := range tests {
		t.Run("invariants", func(t *testing.T) {
			start, end := calculateScrollWindow(tt.selectedLine, tt.totalLines, tt.visibleHeight)

			if start < 0 {
				t.Errorf("selectedLine=%d, totalLines=%d, visibleHeight=%d: start=%d is negative", tt.selectedLine, tt.totalLines, tt.visibleHeight, start)
			}
			if end > tt.totalLines {
				t.Errorf("selectedLine=%d, totalLines=%d, visibleHeight=%d: end=%d exceeds totalLines=%d", tt.selectedLine, tt.totalLines, tt.visibleHeight, end, tt.totalLines)
			}
			if end < start {
				t.Errorf("selectedLine=%d, totalLines=%d, visibleHeight=%d: end=%d < start=%d", tt.selectedLine, tt.totalLines, tt.visibleHeight, end, start)
			}

			windowSize := end - start
			if windowSize > tt.visibleHeight {
				t.Errorf("selectedLine=%d, totalLines=%d, visibleHeight=%d: window size %d exceeds visible height", tt.selectedLine, tt.totalLines, tt.visibleHeight, windowSize)
			}
		})
	}
}
