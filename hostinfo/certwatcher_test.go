package hostinfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func verifyNoMoreEvents(cw *CertWatcher, t *testing.T) {
	select {
	case event := <-cw.Event:
		t.Errorf("Unexpected event received: %v", event)
	case <-time.After(CertWatcherDelay + 5*time.Millisecond):
		return
	}
}

func verifyWriteEvent(cw *CertWatcher, t *testing.T) {
	select {
	case event := <-cw.Event:
		if event != WriteEvent {
			t.Errorf("Expected WriteEvent, got %v", event)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for WriteEvent")
	}
}

func verifyRemoveEvent(cw *CertWatcher, t *testing.T) {
	select {
	case event := <-cw.Event:
		if event != RemoveEvent {
			t.Errorf("Expected RemoveEvent, got %v", event)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for RemoveEvent")
	}
}

func TestReportWriteEvent(t *testing.T) {
	cw := &CertWatcher{Event: make(chan CertEvent, 1)}

	// test that multiple events within the delay window are ignored
	cw.reportWriteEvent()
	cw.reportWriteEvent()
	cw.reportWriteEvent()
	verifyWriteEvent(cw, t)
	verifyNoMoreEvents(cw, t)
}

func TestReportRemoveEvent(t *testing.T) {
	cw := &CertWatcher{Event: make(chan CertEvent, 1)}

	// test that multiple events within the delay window are ignored
	cw.reportRemoveEvent()
	cw.reportRemoveEvent()
	cw.reportRemoveEvent()
	verifyRemoveEvent(cw, t)
	verifyNoMoreEvents(cw, t)
}

// TestCertWatcher tests expected usage of CertWatcher
func TestCertWatcher(t *testing.T) {
	// Create a temporary directory to hold the test certificate file
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "test.crt")

	// Create a new CertWatcher for the test certificate file
	cw, err := NewCertWatcher(certPath)
	if err != nil {
		t.Fatalf("Failed to create CertWatcher: %v", err)
	}

	// Create a test certificate file in the temporary directory
	if err := os.WriteFile(certPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test certificate file: %v", err)
	}
	verifyWriteEvent(cw, t)
	verifyNoMoreEvents(cw, t)

	// Write to the test certificate file and verify that a WriteEvent is received
	if err := os.WriteFile(certPath, []byte("test2"), 0644); err != nil {
		t.Fatalf("Failed to write to test certificate file: %v", err)
	}
	verifyWriteEvent(cw, t)
	verifyNoMoreEvents(cw, t)

	// Verify that certwatch notifies RemoveEvent when certificate is renamed
	newName := filepath.Join(tmpDir, "test2.crt")
	if err := os.Rename(certPath, newName); err != nil {
		t.Fatalf("Failed to rename test certificate file: %v", err)
	}
	verifyRemoveEvent(cw, t)
	verifyNoMoreEvents(cw, t)

	// Verify that certwatch notifies WriteEvent when certificate is renamed back
	if err := os.Rename(newName, certPath); err != nil {
		t.Fatalf("Failed to rename test certificate file: %v", err)
	}
	verifyWriteEvent(cw, t)
	verifyNoMoreEvents(cw, t)

	// Remove the test certificate file and verify that a RemoveEvent is received
	if err := os.Remove(certPath); err != nil {
		t.Fatalf("Failed to remove test certificate file: %v", err)
	}
	verifyRemoveEvent(cw, t)
	verifyNoMoreEvents(cw, t)

	// Verify that the CertWatcher can be closed
	cw.Close()
}
