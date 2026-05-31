// Existing package context for metrics bootstrap.
package metrics

type Observer struct {
	name string
}

func (o *Observer) Observe(value float64, labels string) error {
	// TODO: parse labels then record
	return nil
}
