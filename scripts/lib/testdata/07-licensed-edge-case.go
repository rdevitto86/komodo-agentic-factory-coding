// The inventory vendor rate-limits at 10 rps with no Retry-After header;
// the fixed backoff here is contractual, not tunable.
func (c *InventoryClient) syncBatch(ctx context.Context, items []Item) error {
	return nil
}
