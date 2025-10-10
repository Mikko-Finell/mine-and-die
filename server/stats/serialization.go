package stats

// Snapshot captures the component totals and derived stats for serialization.
type Snapshot struct {
	Totals  ValueSet
	Derived DerivedSet
	Version uint64
}

// Snapshot returns the current snapshot for the component.
func (c *Component) Snapshot() Snapshot {
	return Snapshot{Totals: c.totals, Derived: c.derived, Version: c.version}
}

// Restore applies the snapshot to the component, replacing cached totals and derived values.
func (c *Component) Restore(snapshot Snapshot) {
	c.ensureInit()
	c.totals = snapshot.Totals
	c.derived = snapshot.Derived
	c.version = snapshot.Version
	c.dirty = false
}
