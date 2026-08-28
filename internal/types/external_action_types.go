package types

type ExternalActionID string

const (
	ExternalActionZero ExternalActionID = "" // TS literal 0n: no external action

	ExternalActionTransact            ExternalActionID = "Transact"
	ExternalActionLifi                ExternalActionID = "Lifi"
	ExternalActionOkx                 ExternalActionID = "Okx"
	ExternalActionEmporium            ExternalActionID = "Emporium"
	ExternalActionWallet              ExternalActionID = "Wallet"
	ExternalActionDepositOnChainUtxos ExternalActionID = "DepositOnChainUtxos"
	ExternalActionProofLess           ExternalActionID = "ProofLess"
)

type ExternalActionData struct {
	ExternalActionID       ExternalActionID `json:"externalActionId"`
	ExternalAddress        string           `json:"externalAddress"`
	ExternalActionMetadata string           `json:"externalActionMetadata"`
}
