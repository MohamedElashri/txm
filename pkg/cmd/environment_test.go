package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/MohamedElashri/txm/pkg/backend"
	"github.com/MohamedElashri/txm/pkg/logger"
)

type failedCreate struct {
	*backend.NativeBackend
	observed string
}

func (b *failedCreate) CreateSession(string, ...string) error {
	b.observed = os.Getenv("TXM_LOG_FILE")
	return errors.New("fixture failure")
}

func TestCreateRestoresEnvironmentOnFailure(t *testing.T) {
	t.Setenv("TXM_LOG_FILE", "original")
	t.Setenv("TXM_SESSION_PREFIX", "")
	oldManager, oldLogger, oldFlag := manager, logInstance, createLogFile
	t.Cleanup(func() { manager, logInstance, createLogFile = oldManager, oldLogger, oldFlag })
	fake := &failedCreate{NativeBackend: backend.NewNativeBackend()}
	manager = &backend.Manager{Backend: fake}
	logInstance = logger.NewLogger(0)
	createLogFile = "requested"
	if err := createCmd.RunE(createCmd, []string{"fixture"}); err == nil {
		t.Fatal("failure was swallowed")
	}
	if fake.observed != "requested" || os.Getenv("TXM_LOG_FILE") != "original" {
		t.Fatal("log environment leaked or not applied")
	}
}

func TestCompletionGenerationReportsWriterFailure(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	defer func() { _ = writer.Close() }()
	old, oldInstall := os.Stdout, completionInstall
	os.Stdout = writer
	completionInstall = false
	defer func() { os.Stdout = old; completionInstall = oldInstall }()
	for _, command := range completionCmd.Commands() {
		if command.Name() == "bash" || command.Name() == "zsh" || command.Name() == "fish" {
			if err := command.RunE(command, nil); err == nil {
				t.Errorf("%s generation swallowed writer error", command.Name())
			}
		}
	}
}
