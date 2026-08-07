package types

type DepositAndWithdrawPhase string

const (
	DepositAndWithdrawPhaseBeforeDeposit          DepositAndWithdrawPhase = "before_deposit"
	DepositAndWithdrawPhaseAfterDeposit           DepositAndWithdrawPhase = "after_deposit"
	DepositAndWithdrawPhaseBeforeScheduleWithdraw DepositAndWithdrawPhase = "before_schedule_withdraw"
	DepositAndWithdrawPhaseAfterScheduleWithdraw  DepositAndWithdrawPhase = "after_schedule_withdraw"
)

type DepositAndSendExtendedResult struct {
	DepositTxHash string `json:"depositTxHash"`
	ScheduleID    string `json:"scheduleId"`
}

type ScheduledTransactionStatus string

const (
	ScheduledTransactionStatusPending           ScheduledTransactionStatus = "pending"
	ScheduledTransactionStatusProcessing        ScheduledTransactionStatus = "processing"
	ScheduledTransactionStatusWaitingForRelayer ScheduledTransactionStatus = "waiting_for_relayer"
	ScheduledTransactionStatusSentOnChain       ScheduledTransactionStatus = "sent_on_chain"
	ScheduledTransactionStatusCompleted         ScheduledTransactionStatus = "completed"
	ScheduledTransactionStatusFailed            ScheduledTransactionStatus = "failed"
)

type ScheduledTransactionItemStatus struct {
	Status        ScheduledTransactionStatus `json:"status"`
	ScheduledTime string                     `json:"scheduledTime"`
	TxHash        *string                    `json:"txHash"`
}

type ScheduledTransactionByIDResponse struct {
	ScheduleID   string                           `json:"scheduleId"`
	ChainID      int                              `json:"chainId"`
	Transactions []ScheduledTransactionItemStatus `json:"transactions"`
}
