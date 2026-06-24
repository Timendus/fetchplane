package floatplane

type AuthWait struct{}

func (j AuthWait) Kind() Kind {
	return KindAuthWait
}

func (j AuthWait) Target() string {
	return "login"
}

func (j AuthWait) Run(s *Service, ctx *jobContext) error {
	ctx.SetDetails("waiting for user approval")
	return s.waitForPendingAuth()
}
