package transactions

func proofBatchSize(generateProofRemotely bool) int {
	if generateProofRemotely {
		return 5
	}
	return 1
}
