package ingitdb

import (
	"errors"
	"sync"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ve   ValidationError
		want string
	}{
		{
			name: "error_with_wrapped_error",
			ve: ValidationError{
				Severity: SeverityError,
				Message:  "invalid field value",
				Err:      errors.New("value too large"),
			},
			want: "error: invalid field value: value too large",
		},
		{
			name: "error_without_wrapped_error",
			ve: ValidationError{
				Severity: SeverityError,
				Message:  "missing required field",
			},
			want: "error: missing required field",
		},
		{
			name: "warning_with_wrapped_error",
			ve: ValidationError{
				Severity: SeverityWarning,
				Message:  "deprecated field used",
				Err:      errors.New("use new_field instead"),
			},
			want: "warning: deprecated field used: use new_field instead",
		},
		{
			name: "warning_without_wrapped_error",
			ve: ValidationError{
				Severity: SeverityWarning,
				Message:  "field may be obsolete",
			},
			want: "warning: field may be obsolete",
		},
		{
			name: "error_with_all_location_fields",
			ve: ValidationError{
				Severity:     SeverityError,
				CollectionID: "todo.tasks",
				FilePath:     "/path/to/tasks/task-1.json",
				RecordKey:    "task-1",
				FieldName:    "priority",
				Message:      "invalid priority value",
			},
			want: "error: invalid priority value",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.ve.Error()
			if got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidationResult_Append(t *testing.T) {
	t.Parallel()

	vr := &ValidationResult{}
	
	err1 := ValidationError{
		Severity: SeverityError,
		Message:  "first error",
	}
	err2 := ValidationError{
		Severity: SeverityWarning,
		Message:  "first warning",
	}

	vr.Append(err1)
	vr.Append(err2)

	errs := vr.Errors()
	if len(errs) != 2 {
		t.Fatalf("Errors() length = %d, want 2", len(errs))
	}
	if errs[0].Message != "first error" {
		t.Errorf("Errors()[0].Message = %q, want %q", errs[0].Message, "first error")
	}
	if errs[1].Message != "first warning" {
		t.Errorf("Errors()[1].Message = %q, want %q", errs[1].Message, "first warning")
	}
}

func TestValidationResult_Append_Concurrent(t *testing.T) {
	t.Parallel()

	vr := &ValidationResult{}
	var wg sync.WaitGroup
	
	// Spawn multiple goroutines to test thread safety
	numGoroutines := 10
	errorsPerGoroutine := 100
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < errorsPerGoroutine; j++ {
				vr.Append(ValidationError{
					Severity: SeverityError,
					Message:  "concurrent error",
				})
			}
		}(i)
	}
	
	wg.Wait()
	
	expectedCount := numGoroutines * errorsPerGoroutine
	actualCount := vr.ErrorCount()
	if actualCount != expectedCount {
		t.Errorf("ErrorCount() = %d, want %d", actualCount, expectedCount)
	}
}

func TestValidationResult_Errors(t *testing.T) {
	t.Parallel()

	vr := &ValidationResult{}
	
	// Test empty result
	errs := vr.Errors()
	if len(errs) != 0 {
		t.Errorf("Errors() for empty result length = %d, want 0", len(errs))
	}
	
	// Add errors
	err1 := ValidationError{Severity: SeverityError, Message: "error1"}
	err2 := ValidationError{Severity: SeverityWarning, Message: "warning1"}
	vr.Append(err1)
	vr.Append(err2)
	
	// Get errors
	errs = vr.Errors()
	if len(errs) != 2 {
		t.Fatalf("Errors() length = %d, want 2", len(errs))
	}
	
	// Verify it returns a copy (modifying returned slice shouldn't affect original)
	errs[0].Message = "modified"
	errsAgain := vr.Errors()
	if errsAgain[0].Message == "modified" {
		t.Errorf("Errors() returns a reference instead of a copy")
	}
	if errsAgain[0].Message != "error1" {
		t.Errorf("Errors()[0].Message = %q, want %q", errsAgain[0].Message, "error1")
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	t.Parallel()

	vr := &ValidationResult{}
	
	// Initially empty
	if vr.HasErrors() {
		t.Error("HasErrors() = true for empty result, want false")
	}
	
	// After adding an error
	vr.Append(ValidationError{Severity: SeverityError, Message: "error"})
	if !vr.HasErrors() {
		t.Error("HasErrors() = false after adding error, want true")
	}
	
	// After adding a warning (should still be true)
	vr.Append(ValidationError{Severity: SeverityWarning, Message: "warning"})
	if !vr.HasErrors() {
		t.Error("HasErrors() = false after adding warning, want true")
	}
}

func TestValidationResult_ErrorCount(t *testing.T) {
	t.Parallel()

	vr := &ValidationResult{}
	
	// Initially zero
	if count := vr.ErrorCount(); count != 0 {
		t.Errorf("ErrorCount() = %d for empty result, want 0", count)
	}
	
	// Add errors incrementally
	for i := 1; i <= 5; i++ {
		vr.Append(ValidationError{
			Severity: SeverityError,
			Message:  "error",
		})
		if count := vr.ErrorCount(); count != i {
			t.Errorf("ErrorCount() = %d after adding %d errors, want %d", count, i, i)
		}
	}
}

func TestValidationResult_Filter(t *testing.T) {
	t.Parallel()

	vr := &ValidationResult{}
	
	err1 := ValidationError{Severity: SeverityError, Message: "error1", CollectionID: "tasks"}
	err2 := ValidationError{Severity: SeverityWarning, Message: "warning1", CollectionID: "tasks"}
	err3 := ValidationError{Severity: SeverityError, Message: "error2", CollectionID: "users"}
	err4 := ValidationError{Severity: SeverityWarning, Message: "warning2", CollectionID: "users"}
	
	vr.Append(err1)
	vr.Append(err2)
	vr.Append(err3)
	vr.Append(err4)

	tests := []struct {
		name      string
		predicate func(ValidationError) bool
		wantLen   int
		wantFirst string
	}{
		{
			name:      "filter_errors_only",
			predicate: func(e ValidationError) bool { return e.Severity == SeverityError },
			wantLen:   2,
			wantFirst: "error1",
		},
		{
			name:      "filter_warnings_only",
			predicate: func(e ValidationError) bool { return e.Severity == SeverityWarning },
			wantLen:   2,
			wantFirst: "warning1",
		},
		{
			name:      "filter_by_collection",
			predicate: func(e ValidationError) bool { return e.CollectionID == "tasks" },
			wantLen:   2,
			wantFirst: "error1",
		},
		{
			name:      "filter_specific_combination",
			predicate: func(e ValidationError) bool { 
				return e.Severity == SeverityError && e.CollectionID == "users"
			},
			wantLen:   1,
			wantFirst: "error2",
		},
		{
			name:      "filter_nothing_matches",
			predicate: func(e ValidationError) bool { return e.CollectionID == "nonexistent" },
			wantLen:   0,
			wantFirst: "",
		},
		{
			name:      "filter_all_match",
			predicate: func(e ValidationError) bool { return true },
			wantLen:   4,
			wantFirst: "error1",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			filtered := vr.Filter(tc.predicate)
			if len(filtered) != tc.wantLen {
				t.Fatalf("Filter() returned %d errors, want %d", len(filtered), tc.wantLen)
			}
			if tc.wantLen > 0 && filtered[0].Message != tc.wantFirst {
				t.Errorf("Filter()[0].Message = %q, want %q", filtered[0].Message, tc.wantFirst)
			}
		})
	}
}

func TestValidationResult_Filter_EmptyResult(t *testing.T) {
	t.Parallel()

	vr := &ValidationResult{}
	
	filtered := vr.Filter(func(e ValidationError) bool { return true })
	if len(filtered) != 0 {
		t.Errorf("Filter() on empty result returned %d errors, want 0", len(filtered))
	}
}

func TestValidationError_AllFields(t *testing.T) {
	t.Parallel()

	// Test that all fields are properly stored
	ve := ValidationError{
		Severity:     SeverityError,
		CollectionID: "todo.tasks",
		FilePath:     "/absolute/path/to/tasks/task-1.json",
		RecordKey:    "task-1",
		FieldName:    "priority",
		Message:      "invalid value",
		Err:          errors.New("underlying cause"),
	}
	
	if ve.Severity != SeverityError {
		t.Errorf("Severity = %q, want %q", ve.Severity, SeverityError)
	}
	if ve.CollectionID != "todo.tasks" {
		t.Errorf("CollectionID = %q, want %q", ve.CollectionID, "todo.tasks")
	}
	if ve.FilePath != "/absolute/path/to/tasks/task-1.json" {
		t.Errorf("FilePath = %q, want %q", ve.FilePath, "/absolute/path/to/tasks/task-1.json")
	}
	if ve.RecordKey != "task-1" {
		t.Errorf("RecordKey = %q, want %q", ve.RecordKey, "task-1")
	}
	if ve.FieldName != "priority" {
		t.Errorf("FieldName = %q, want %q", ve.FieldName, "priority")
	}
	if ve.Message != "invalid value" {
		t.Errorf("Message = %q, want %q", ve.Message, "invalid value")
	}
	if ve.Err == nil || ve.Err.Error() != "underlying cause" {
		t.Errorf("Err = %v, want error with message 'underlying cause'", ve.Err)
	}
}

func TestSeverityConstants(t *testing.T) {
	t.Parallel()

	// Test that severity constants have expected values
	if SeverityError != "error" {
		t.Errorf("SeverityError = %q, want %q", SeverityError, "error")
	}
	if SeverityWarning != "warning" {
		t.Errorf("SeverityWarning = %q, want %q", SeverityWarning, "warning")
	}
}
