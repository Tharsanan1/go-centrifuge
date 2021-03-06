package nft

import (
	"context"

	"github.com/centrifuge/go-centrifuge/centerrors"
	"github.com/centrifuge/go-centrifuge/code"
	"github.com/centrifuge/go-centrifuge/config"
	"github.com/centrifuge/go-centrifuge/contextutil"
	"github.com/centrifuge/go-centrifuge/protobufs/gen/go/nft"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	logging "github.com/ipfs/go-log"
)

var apiLog = logging.Logger("nft-api")

type grpcHandler struct {
	config  config.Service
	service InvoiceUnpaid
}

// GRPCHandler returns an implementation of invoice.DocumentServiceServer
func GRPCHandler(config config.Service, InvoiceUnpaid InvoiceUnpaid) nftpb.NFTServiceServer {
	return &grpcHandler{config: config, service: InvoiceUnpaid}
}

// MintNFT will be called from the client API to mint an NFT
func (g grpcHandler) MintNFT(ctx context.Context, request *nftpb.NFTMintRequest) (*nftpb.NFTMintResponse, error) {
	apiLog.Infof("Received request to Mint an NFT with  %s with proof fields %s", request.Identifier, request.ProofFields)
	ctxHeader, err := contextutil.Context(ctx, g.config)
	if err != nil {
		apiLog.Error(err)
		return nil, err
	}

	err = validateParameters(request)
	if err != nil {
		return nil, err
	}

	identifier, err := hexutil.Decode(request.Identifier)
	if err != nil {
		return nil, err
	}

	req := MintNFTRequest{
		DocumentID:               identifier,
		RegistryAddress:          common.HexToAddress(request.RegistryAddress),
		DepositAddress:           common.HexToAddress(request.DepositAddress),
		ProofFields:              request.ProofFields,
		GrantNFTReadAccess:       request.GrantNftAccess,
		SubmitNFTReadAccessProof: request.SubmitNftOwnerAccessProof,
		SubmitTokenProof:         request.SubmitTokenProof,
	}
	resp, _, err := g.service.MintNFT(ctxHeader, req)
	if err != nil {
		return nil, centerrors.New(code.Unknown, err.Error())
	}

	return &nftpb.NFTMintResponse{
		Header: &nftpb.ResponseHeader{JobId: resp.JobID},
	}, nil
}

// MintInvoiceUnpaidNFT will be called from the client API to mint an NFT out of an unpaid invoice
func (g grpcHandler) MintInvoiceUnpaidNFT(ctx context.Context, request *nftpb.NFTMintInvoiceUnpaidRequest) (*nftpb.NFTMintResponse, error) {
	apiLog.Infof("Received request to Mint an Invoice Unpaid NFT for invoice %s and deposit address %s", request.Identifier, request.DepositAddress)
	ctxHeader, err := contextutil.Context(ctx, g.config)
	if err != nil {
		apiLog.Error(err)
		return nil, err
	}

	// Get proof fields
	proofFields, err := g.service.GetRequiredInvoiceUnpaidProofFields(ctxHeader)
	if err != nil {
		return nil, centerrors.New(code.Unknown, err.Error())
	}

	cfg, err := g.config.GetConfig()
	if err != nil {
		return nil, centerrors.New(code.Unknown, err.Error())
	}
	poRegistry := cfg.GetContractAddress(config.InvoiceUnpaidNFT)

	mintReq := &nftpb.NFTMintRequest{
		Identifier:                request.Identifier,
		DepositAddress:            request.DepositAddress,
		RegistryAddress:           poRegistry.Hex(),
		ProofFields:               proofFields,
		GrantNftAccess:            true,
		SubmitNftOwnerAccessProof: true,
		SubmitTokenProof:          true,
	}

	return g.MintNFT(ctx, mintReq)
}

func validateParameters(request *nftpb.NFTMintRequest) error {
	if !common.IsHexAddress(request.RegistryAddress) {
		return centerrors.New(code.Unknown, "registryAddress is not a valid Ethereum address")
	}

	if !common.IsHexAddress(request.DepositAddress) {
		return centerrors.New(code.Unknown, "DepositAddress is not a valid Ethereum address")
	}

	return nil
}
