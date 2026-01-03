package ingitdb

import "testing"

func TestValidate(t *testing.T) {
	t.Run("fail_if_no_root_config_file", func(t *testing.T) {
		_, err := ReadDefinition(".")
		if err == nil {
			t.Fatal("expected error, got none")
		}
	})
}
