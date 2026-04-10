package adapter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	computebudget "github.com/gagliardetto/solana-go/programs/compute-budget"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/solock/solock/internal/domain"
)

const (
	computeUnitLimit    uint32 = 200_000
	fallbackPriorityFee uint64 = 10_000 // micro-lamports per CU
)

type SolanaVaultRepo struct {
	rpc            *rpc.Client
	owner          solana.PrivateKey
	programID      solana.PublicKey
	programKeypair solana.PrivateKey
	rpcURL         string
}

func NewSolanaVaultRepo(keys *domain.DerivedKeys, rpcURL string) domain.VaultRepository {
	return &SolanaVaultRepo{
		rpc:            rpc.New(rpcURL),
		owner:          solana.PrivateKey(keys.DeployerKeypair),
		programID:      solana.PublicKeyFromBytes(keys.ProgramKeypair[32:]),
		programKeypair: solana.PrivateKey(keys.ProgramKeypair),
		rpcURL:         rpcURL,
	}
}

func (r *SolanaVaultRepo) vaultPDA() (solana.PublicKey, uint8, error) {
	return solana.FindProgramAddress(
		[][]byte{[]byte("vault_v2"), r.owner.PublicKey().Bytes()},
		r.programID,
	)
}

func (r *SolanaVaultRepo) entryPDA(index uint32) (solana.PublicKey, uint8, error) {
	indexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBytes, index)
	return solana.FindProgramAddress(
		[][]byte{[]byte("entry"), r.owner.PublicKey().Bytes(), indexBytes},
		r.programID,
	)
}

func (r *SolanaVaultRepo) groupPDA(index uint32) (solana.PublicKey, uint8, error) {
	indexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexBytes, index)
	return solana.FindProgramAddress(
		[][]byte{[]byte("group_v2"), r.owner.PublicKey().Bytes(), indexBytes},
		r.programID,
	)
}

func (r *SolanaVaultRepo) discriminator(name string) []byte {
	h := sha256.Sum256([]byte("global:" + name))
	return h[:8]
}

func (r *SolanaVaultRepo) Initialize(ctx context.Context) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: r.discriminator("initialize_vault"),
	})
}

func (r *SolanaVaultRepo) Reset(ctx context.Context) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: r.discriminator("reset_vault"),
	})
}

func (r *SolanaVaultRepo) Exists(ctx context.Context) (bool, error) {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return false, err
	}
	info, err := r.rpc.GetAccountInfo(ctx, vaultPDA)
	if err != nil {
		return false, fmt.Errorf("check vault: %w", err)
	}
	return info != nil && info.Value != nil, nil
}

func (r *SolanaVaultRepo) GetMeta(ctx context.Context) (*domain.VaultMeta, error) {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return nil, err
	}
	info, err := r.rpc.GetAccountInfo(ctx, vaultPDA)
	if err != nil || info == nil || info.Value == nil {
		return nil, errors.New("vault not initialized")
	}
	return r.parseVaultMeta(info.Value.Data.GetBinary())
}

func (r *SolanaVaultRepo) AddEntry(ctx context.Context, index uint32, data []byte) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	entryPDA, _, err := r.entryPDA(index)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.Write(r.discriminator("add_entry"))
	binary.Write(&buf, binary.LittleEndian, index)
	binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
	buf.Write(data)

	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: entryPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: buf.Bytes(),
	})
}

func (r *SolanaVaultRepo) UpdateEntry(ctx context.Context, index uint32, data []byte, expectedUpdatedAt int64) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	entryPDA, _, err := r.entryPDA(index)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.Write(r.discriminator("update_entry"))
	binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
	buf.Write(data)
	binary.Write(&buf, binary.LittleEndian, expectedUpdatedAt)

	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: entryPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: buf.Bytes(),
	})
}

func (r *SolanaVaultRepo) DeleteEntry(ctx context.Context, index uint32) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	entryPDA, _, err := r.entryPDA(index)
	if err != nil {
		return err
	}

	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: entryPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: r.discriminator("delete_entry"),
	})
}

func (r *SolanaVaultRepo) AddGroup(ctx context.Context, index uint32, data []byte) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	groupPDA, _, err := r.groupPDA(index)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.Write(r.discriminator("add_group"))
	binary.Write(&buf, binary.LittleEndian, index)
	binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
	buf.Write(data)

	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: groupPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: buf.Bytes(),
	})
}

func (r *SolanaVaultRepo) UpdateGroup(ctx context.Context, index uint32, data []byte) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	groupPDA, _, err := r.groupPDA(index)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.Write(r.discriminator("update_group"))
	binary.Write(&buf, binary.LittleEndian, uint32(len(data)))
	buf.Write(data)

	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: false},
			{PublicKey: groupPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: buf.Bytes(),
	})
}

func (r *SolanaVaultRepo) DeleteGroup(ctx context.Context, index uint32) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	groupPDA, _, err := r.groupPDA(index)
	if err != nil {
		return err
	}

	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: true},
			{PublicKey: groupPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: r.discriminator("delete_group"),
	})
}

func (r *SolanaVaultRepo) PurgeGroup(ctx context.Context, index uint32) error {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return err
	}
	groupPDA, _, err := r.groupPDA(index)
	if err != nil {
		return err
	}

	return r.send(ctx, &solana.GenericInstruction{
		ProgID: r.programID,
		AccountValues: solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: vaultPDA, IsSigner: false, IsWritable: false},
			{PublicKey: groupPDA, IsSigner: false, IsWritable: true},
			{PublicKey: solana.SystemProgramID, IsSigner: false, IsWritable: false},
		},
		DataBytes: r.discriminator("purge_group"),
	})
}

func (r *SolanaVaultRepo) GetGroup(ctx context.Context, index uint32) (*domain.GroupAccount, error) {
	pda, _, err := r.groupPDA(index)
	if err != nil {
		return nil, err
	}
	info, err := r.rpc.GetAccountInfo(ctx, pda)
	if err != nil || info == nil || info.Value == nil {
		return nil, nil
	}
	return r.parseGroupAccount(info.Value.Data.GetBinary(), index)
}

func (r *SolanaVaultRepo) GetGroupsBatch(ctx context.Context, indices []uint32) (map[uint32]*domain.GroupAccount, error) {
	if len(indices) == 0 {
		return nil, nil
	}

	pubkeys := make([]solana.PublicKey, len(indices))
	for i, idx := range indices {
		pda, _, err := r.groupPDA(idx)
		if err != nil {
			return nil, err
		}
		pubkeys[i] = pda
	}

	result, err := r.rpc.GetMultipleAccounts(ctx, pubkeys...)
	if err != nil {
		return nil, err
	}

	groups := make(map[uint32]*domain.GroupAccount)
	for i, acc := range result.Value {
		if acc == nil {
			continue
		}
		group, err := r.parseGroupAccount(acc.Data.GetBinary(), indices[i])
		if err != nil {
			continue
		}
		groups[indices[i]] = group
	}
	return groups, nil
}

func (r *SolanaVaultRepo) GetEntry(ctx context.Context, index uint32) (*domain.EntryAccount, error) {
	pda, _, err := r.entryPDA(index)
	if err != nil {
		return nil, err
	}
	info, err := r.rpc.GetAccountInfo(ctx, pda)
	if err != nil || info == nil || info.Value == nil {
		return nil, nil
	}
	return r.parseEntryAccount(info.Value.Data.GetBinary())
}

func (r *SolanaVaultRepo) GetEntriesBatch(ctx context.Context, indices []uint32) (map[uint32]*domain.EntryAccount, error) {
	if len(indices) == 0 {
		return nil, nil
	}

	pubkeys := make([]solana.PublicKey, len(indices))
	for i, idx := range indices {
		pda, _, err := r.entryPDA(idx)
		if err != nil {
			return nil, err
		}
		pubkeys[i] = pda
	}

	result, err := r.rpc.GetMultipleAccounts(ctx, pubkeys...)
	if err != nil {
		return nil, err
	}

	entries := make(map[uint32]*domain.EntryAccount)
	for i, acc := range result.Value {
		if acc == nil {
			continue
		}
		entry, err := r.parseEntryAccount(acc.Data.GetBinary())
		if err != nil {
			continue
		}
		entries[indices[i]] = entry
	}
	return entries, nil
}

func (r *SolanaVaultRepo) IsProgramDeployed(ctx context.Context) (bool, error) {
	info, err := r.rpc.GetAccountInfo(ctx, r.programID)
	if err != nil {
		return false, fmt.Errorf("check program: %w", err)
	}
	return info != nil && info.Value != nil && info.Value.Executable, nil
}

func (r *SolanaVaultRepo) GetBalance(ctx context.Context) (uint64, error) {
	result, err := r.rpc.GetBalance(ctx, r.owner.PublicKey(), rpc.CommitmentConfirmed)
	if err != nil {
		return 0, err
	}
	return result.Value, nil
}

func (r *SolanaVaultRepo) DeployProgram(ctx context.Context, programBinary []byte) error {
	if len(programBinary) == 0 {
		return fmt.Errorf("program binary is empty")
	}

	tmpDir, err := os.MkdirTemp("", "solock-deploy-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	soPath := filepath.Join(tmpDir, "solock.so")
	if err := os.WriteFile(soPath, programBinary, 0600); err != nil {
		return fmt.Errorf("write program binary: %w", err)
	}

	deployerPath := filepath.Join(tmpDir, "deployer.json")

	if err := writeKeypairJSON(deployerPath, r.owner); err != nil {
		return err
	}

	solanaBin, err := exec.LookPath("solana")
	if err != nil {
		return fmt.Errorf("solana CLI not found: %w", err)
	}

	deployed, _ := r.IsProgramDeployed(ctx)
	var programArg string
	if deployed {
		programArg = r.programID.String()
	} else {
		programPath := filepath.Join(tmpDir, "program.json")
		if err := writeKeypairJSON(programPath, r.programKeypair); err != nil {
			return err
		}
		programArg = programPath
	}

	cmd := exec.CommandContext(ctx, solanaBin, "program", "deploy",
		"--program-id", programArg,
		"--keypair", deployerPath,
		soPath,
		"--url", r.rpcURL,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("deploy: %s", msg)
	}
	return nil
}

func (r *SolanaVaultRepo) CloseProgram(ctx context.Context) error {
	tmpDir, err := os.MkdirTemp("", "solock-close-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	deployerPath := filepath.Join(tmpDir, "deployer.json")
	if err := writeKeypairJSON(deployerPath, r.owner); err != nil {
		return err
	}

	solanaBin, err := exec.LookPath("solana")
	if err != nil {
		return fmt.Errorf("solana CLI not found: %w", err)
	}

	cmd := exec.CommandContext(ctx, solanaBin, "program", "close",
		r.programID.String(),
		"--keypair", deployerPath,
		"--url", r.rpcURL,
		"--bypass-warning",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if len(msg) > 500 {
			msg = msg[:500]
		}
		return fmt.Errorf("close: %s", msg)
	}
	return nil
}

func (r *SolanaVaultRepo) TransferAll(ctx context.Context, to string) error {
	toPubkey, err := solana.PublicKeyFromBase58(to)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	balance, err := r.GetBalance(ctx)
	if err != nil {
		return fmt.Errorf("get balance: %w", err)
	}
	if balance == 0 {
		return fmt.Errorf("balance is zero")
	}

	fee := uint64(5000)
	if balance <= fee {
		return fmt.Errorf("balance too low to cover fee")
	}
	amount := balance - fee

	ix := solana.NewInstruction(
		solana.SystemProgramID,
		solana.AccountMetaSlice{
			{PublicKey: r.owner.PublicKey(), IsSigner: true, IsWritable: true},
			{PublicKey: toPubkey, IsSigner: false, IsWritable: true},
		},
		// SystemProgram::Transfer instruction data: [2, 0, 0, 0] + le_u64(amount)
		transferInstructionData(amount),
	)

	return r.send(ctx, ix)
}

func transferInstructionData(lamports uint64) []byte {
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], 2) // Transfer instruction index
	binary.LittleEndian.PutUint64(data[4:12], lamports)
	return data
}

func (r *SolanaVaultRepo) send(ctx context.Context, ix solana.Instruction) error {
	bh, err := r.rpc.GetLatestBlockhash(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return fmt.Errorf("get blockhash: %w", err)
	}

	priorityFee := r.estimatePriorityFee(ctx)
	cuLimitIx := computebudget.NewSetComputeUnitLimitInstruction(computeUnitLimit).Build()
	cuPriceIx := computebudget.NewSetComputeUnitPriceInstruction(priorityFee).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{cuLimitIx, cuPriceIx, ix},
		bh.Value.Blockhash,
		solana.TransactionPayer(r.owner.PublicKey()),
	)
	if err != nil {
		return fmt.Errorf("build tx: %w", err)
	}

	ownerKey := r.owner
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(ownerKey.PublicKey()) {
			return &ownerKey
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sign tx: %w", err)
	}

	sig, err := r.rpc.SendTransactionWithOpts(ctx, tx, rpc.TransactionOpts{
		PreflightCommitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return fmt.Errorf("send transaction: %w", err)
	}

	for range 30 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
		result, err := r.rpc.GetSignatureStatuses(ctx, false, sig)
		if err != nil {
			continue
		}
		if result == nil || len(result.Value) == 0 || result.Value[0] == nil {
			continue
		}
		status := result.Value[0]
		if status.Err != nil {
			return fmt.Errorf("tx failed: %v", status.Err)
		}
		if status.ConfirmationStatus == rpc.ConfirmationStatusConfirmed ||
			status.ConfirmationStatus == rpc.ConfirmationStatusFinalized {
			return nil
		}
	}
	return fmt.Errorf("tx confirmation timeout: %s", sig)
}

func (r *SolanaVaultRepo) estimatePriorityFee(ctx context.Context) uint64 {
	vaultPDA, _, err := r.vaultPDA()
	if err != nil {
		return fallbackPriorityFee
	}

	accounts := solana.PublicKeySlice{vaultPDA}
	fees, err := r.rpc.GetRecentPrioritizationFees(ctx, accounts)
	if err != nil || len(fees) == 0 {
		return fallbackPriorityFee
	}

	samples := make([]uint64, 0, len(fees))
	for _, f := range fees {
		if f.PrioritizationFee > 0 {
			samples = append(samples, f.PrioritizationFee)
		}
	}
	if len(samples) == 0 {
		return fallbackPriorityFee
	}

	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p75 := samples[(len(samples)*75)/100]
	if p75 < fallbackPriorityFee {
		return fallbackPriorityFee
	}
	return p75
}

func (r *SolanaVaultRepo) parseVaultMeta(data []byte) (*domain.VaultMeta, error) {
	if len(data) < 8+32+4+4+1 {
		return nil, errors.New("invalid vault meta")
	}
	d := data[8+32:]
	meta := &domain.VaultMeta{
		NextIndex:  binary.LittleEndian.Uint32(d[:4]),
		EntryCount: binary.LittleEndian.Uint32(d[4:8]),
	}
	d = d[9:]
	if len(d) >= 4 {
		vecLen := binary.LittleEndian.Uint32(d[:4])
		d = d[4:]
		meta.FreeSlots = make([]uint32, 0, vecLen)
		for i := uint32(0); i < vecLen && len(d) >= 4; i++ {
			meta.FreeSlots = append(meta.FreeSlots, binary.LittleEndian.Uint32(d[:4]))
			d = d[4:]
		}
	}
	if len(d) >= 8 {
		meta.NextGroupIndex = binary.LittleEndian.Uint32(d[:4])
		meta.GroupCount = binary.LittleEndian.Uint32(d[4:8])
	}
	return meta, nil
}

func (r *SolanaVaultRepo) parseEntryAccount(data []byte) (*domain.EntryAccount, error) {
	if len(data) < 8+32+4+4 {
		return nil, errors.New("invalid entry account")
	}
	d := data[8+32:]
	index := binary.LittleEndian.Uint32(d[:4])
	d = d[4:]
	vecLen := binary.LittleEndian.Uint32(d[:4])
	d = d[4:]
	if len(d) < int(vecLen) {
		return nil, errors.New("invalid data length")
	}
	encData := make([]byte, vecLen)
	copy(encData, d[:vecLen])
	d = d[vecLen:]
	if len(d) < 17 {
		return nil, errors.New("invalid tail")
	}
	return &domain.EntryAccount{
		Index:         index,
		EncryptedData: encData,
		CreatedAt:     int64(binary.LittleEndian.Uint64(d[:8])),
		UpdatedAt:     int64(binary.LittleEndian.Uint64(d[8:16])),
	}, nil
}

func (r *SolanaVaultRepo) parseGroupAccount(data []byte, index uint32) (*domain.GroupAccount, error) {
	// discriminator(8) + owner(32) + index(4) + vec_len(4) + ... + deleted(1) + bump(1)
	if len(data) < 8+32+4+4 {
		return nil, errors.New("invalid group account")
	}
	d := data[8+32:]
	_ = binary.LittleEndian.Uint32(d[:4]) // index from on-chain
	d = d[4:]
	vecLen := binary.LittleEndian.Uint32(d[:4])
	d = d[4:]
	if len(d) < int(vecLen) {
		return nil, errors.New("invalid group data length")
	}
	encData := make([]byte, vecLen)
	copy(encData, d[:vecLen])
	d = d[vecLen:]
	if len(d) < 2 {
		return nil, errors.New("invalid group tail")
	}
	deleted := d[0] != 0
	return &domain.GroupAccount{
		Index:         index,
		EncryptedData: encData,
		Deleted:       deleted,
	}, nil
}

func writeKeypairJSON(path string, key solana.PrivateKey) error {
	intSlice := make([]int, len(key))
	for i, b := range key {
		intSlice[i] = int(b)
	}
	data, err := json.Marshal(intSlice)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
