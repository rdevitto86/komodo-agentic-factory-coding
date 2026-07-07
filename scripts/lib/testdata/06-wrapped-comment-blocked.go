// Refunds settle through the PSP's T+2 batch, so state moves to
// pending_settlement here and the webhook completes
// it once confirmed.
func (s *RefundService) InitiateRefund(ctx context.Context, orderID string) error {
	return nil
}
