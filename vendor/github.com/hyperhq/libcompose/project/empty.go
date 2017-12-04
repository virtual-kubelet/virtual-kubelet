package project

import (
	"github.com/hyperhq/libcompose/project/options"
)

// EmptyService is a struct that implements Service but does nothing.
type EmptyService struct {
}

// Create implements Service.Create but does nothing.
func (e *EmptyService) Create(options options.Create) error {
	return nil
}

// Build implements Service.Build but does nothing.
func (e *EmptyService) Build(buildOptions options.Build) error {
	return nil
}

// Up implements Service.Up but does nothing.
func (e *EmptyService) Up(options options.Up) error {
	return nil
}

// Start implements Service.Start but does nothing.
func (e *EmptyService) Start() error {
	return nil
}

// Stop implements Service.Stop() but does nothing.
func (e *EmptyService) Stop(timeout int) error {
	return nil
}

// Delete implements Service.Delete but does nothing.
func (e *EmptyService) Delete(options options.Delete) error {
	return nil
}

// Restart implements Service.Restart but does nothing.
func (e *EmptyService) Restart(timeout int) error {
	return nil
}

// Log implements Service.Log but does nothing.
func (e *EmptyService) Log(follow bool) error {
	return nil
}

// Pull implements Service.Pull but does nothing.
func (e *EmptyService) Pull() error {
	return nil
}

// Kill implements Service.Kill but does nothing.
func (e *EmptyService) Kill(signal string) error {
	return nil
}

// Containers implements Service.Containers but does nothing.
func (e *EmptyService) Containers() ([]Container, error) {
	return []Container{}, nil
}

// Scale implements Service.Scale but does nothing.
func (e *EmptyService) Scale(count int, timeout int) error {
	return nil
}

// Info implements Service.Info but does nothing.
func (e *EmptyService) Info(qFlag bool) (InfoSet, error) {
	return InfoSet{}, nil
}

// Pause implements Service.Pause but does nothing.
func (e *EmptyService) Pause() error {
	return nil
}

// Unpause implements Service.Pause but does nothing.
func (e *EmptyService) Unpause() error {
	return nil
}

// Run implements Service.Run but does nothing.
func (e *EmptyService) Run(commandParts []string) (int, error) {
	return 0, nil
}

// RemoveImage implements Service.RemoveImage but does nothing.
func (e *EmptyService) RemoveImage(imageType options.ImageType) error {
	return nil
}
