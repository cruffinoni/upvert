package bump

// runStep reports one real pipeline step and marks it failed when fn fails.
// Previews never report steps because the service uses a no-op progress.
func (s *service) runStep(label, detail string, fn func() error) error {
	s.progress.Start(label)
	if err := fn(); err != nil {
		s.progress.Abort()
		return err
	}
	s.progress.Finish(detail)
	return nil
}
