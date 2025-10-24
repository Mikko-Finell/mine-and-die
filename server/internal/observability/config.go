package observability

// Config captures opt-in observability toggles that wire into the server.
type Config struct {
	EnablePprofTrace bool
}
