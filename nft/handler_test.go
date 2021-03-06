// +build unit

package nft

import (
	"context"
	"math/big"
	"testing"

	"github.com/centrifuge/go-centrifuge/config"
	"github.com/centrifuge/go-centrifuge/config/configstore"
	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/protobufs/gen/go/nft"
	"github.com/centrifuge/go-centrifuge/testingutils/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockInvoiceUnpaid struct {
	mock.Mock
}

func (m *mockInvoiceUnpaid) MintNFT(ctx context.Context, request MintNFTRequest) (*MintNFTResponse, chan bool, error) {
	args := m.Called(ctx, request)
	resp, _ := args.Get(0).(*MintNFTResponse)
	return resp, nil, args.Error(1)
}

func (m *mockInvoiceUnpaid) GetRequiredInvoiceUnpaidProofFields(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	resp, _ := args.Get(0).([]string)
	return resp, args.Error(1)
}

func TestNFTMint_success(t *testing.T) {
	nftMintRequest := getTestSetupData()
	mockService := &mockInvoiceUnpaid{}
	mockConfigStore := mockmockConfigStore()
	docID, err := hexutil.Decode(nftMintRequest.Identifier)
	assert.NoError(t, err)

	tokID := big.NewInt(1)
	nftResponse := &MintNFTResponse{TokenID: tokID.String()}
	req := MintNFTRequest{
		DocumentID:      docID,
		RegistryAddress: common.HexToAddress(nftMintRequest.RegistryAddress),
		DepositAddress:  common.HexToAddress(nftMintRequest.DepositAddress),
		ProofFields:     nftMintRequest.ProofFields,
	}
	mockService.On("MintNFT", mock.Anything, req).Return(nftResponse, nil)
	handler := grpcHandler{mockConfigStore, mockService}
	_, err = handler.MintNFT(testingconfig.HandlerContext(mockConfigStore), nftMintRequest)
	mockService.AssertExpectations(t)
	assert.Nil(t, err, "mint nft should be successful")
}

func TestPaymentObligationNFTMint_success(t *testing.T) {
	mockService := &mockInvoiceUnpaid{}
	mockConfigStore := mockmockConfigStore()
	tokID := big.NewInt(1)
	nftResponse := &MintNFTResponse{TokenID: tokID.String()}
	nftReq := &nftpb.NFTMintInvoiceUnpaidRequest{
		Identifier:     "0x1234567890",
		DepositAddress: "0xf72855759a39fb75fc7341139f5d7a3974d4da08",
	}

	// error no account header
	handler := grpcHandler{mockConfigStore, mockService}
	nftMintResponse, err := handler.MintInvoiceUnpaidNFT(context.Background(), nftReq)
	mockService.AssertExpectations(t)
	assert.Error(t, err)
	assert.Nil(t, nftMintResponse)

	// error generate proofs
	mockService.On("GetRequiredInvoiceUnpaidProofFields", mock.Anything).Return(nil, errors.New("fail")).Once()
	handler = grpcHandler{mockConfigStore, mockService}
	nftMintResponse, err = handler.MintInvoiceUnpaidNFT(testingconfig.HandlerContext(mockConfigStore), nftReq)
	mockService.AssertExpectations(t)
	assert.Error(t, err)
	assert.Nil(t, nftMintResponse)

	// error get config
	mockService.On("GetRequiredInvoiceUnpaidProofFields", mock.Anything).Return([]string{"proof1", "proof2"}, nil).Once()
	mockConfigStore.On("GetConfig").Return(cfg, errors.New("fail")).Once()
	handler = grpcHandler{mockConfigStore, mockService}
	nftMintResponse, err = handler.MintInvoiceUnpaidNFT(testingconfig.HandlerContext(mockConfigStore), nftReq)
	mockService.AssertExpectations(t)
	assert.Error(t, err)
	assert.Nil(t, nftMintResponse)

	// success assertions
	mockService.On("MintNFT", mock.Anything, mock.Anything).Return(nftResponse, nil).Once()
	mockService.On("GetRequiredInvoiceUnpaidProofFields", mock.Anything).Return([]string{"proof1", "proof2"}, nil).Once()
	mockConfigStore.On("GetConfig").Return(cfg, nil).Once()
	handler = grpcHandler{mockConfigStore, mockService}
	nftMintResponse, err = handler.MintInvoiceUnpaidNFT(testingconfig.HandlerContext(mockConfigStore), nftReq)
	mockService.AssertExpectations(t)
	assert.Nil(t, err, "mint nft should be successful")
}

func mockmockConfigStore() *configstore.MockService {
	mockConfigStore := &configstore.MockService{}
	mockConfigStore.On("GetAccount", mock.Anything).Return(&configstore.Account{}, nil)
	mockConfigStore.On("GetAllAccounts").Return([]config.Account{&configstore.Account{}}, nil)
	return mockConfigStore
}

func TestNFTMint_InvalidIdentifier(t *testing.T) {
	nftMintRequest := getTestSetupData()
	nftMintRequest.Identifier = "32321"
	mockConfigStore := mockmockConfigStore()
	mockConfigStore.On("GetAllAccounts").Return(testingconfig.HandlerContext(mockConfigStore))
	handler := grpcHandler{mockConfigStore, &mockInvoiceUnpaid{}}
	_, err := handler.MintNFT(testingconfig.HandlerContext(mockConfigStore), nftMintRequest)
	assert.Error(t, err, "invalid identifier should throw an error")
}

func TestNFTMint_ServiceError(t *testing.T) {
	nftMintRequest := getTestSetupData()
	mockService := &mockInvoiceUnpaid{}
	docID, err := hexutil.Decode(nftMintRequest.Identifier)
	assert.NoError(t, err)
	req := MintNFTRequest{
		DocumentID:      docID,
		RegistryAddress: common.HexToAddress(nftMintRequest.RegistryAddress),
		DepositAddress:  common.HexToAddress(nftMintRequest.DepositAddress),
		ProofFields:     nftMintRequest.ProofFields,
	}

	mockService.On("MintNFT", mock.Anything, req).Return(nil, errors.New("service error"))
	mockConfigStore := mockmockConfigStore()
	handler := grpcHandler{mockConfigStore, mockService}
	_, err = handler.MintNFT(testingconfig.HandlerContext(mockConfigStore), nftMintRequest)
	mockService.AssertExpectations(t)
	assert.NotNil(t, err)
}

func TestNFTMint_InvalidAddresses(t *testing.T) {
	nftMintRequest := getTestSetupData()
	nftMintRequest.RegistryAddress = "0x1234"
	mockConfigStore := mockmockConfigStore()
	handler := grpcHandler{mockConfigStore, &mockInvoiceUnpaid{}}
	_, err := handler.MintNFT(testingconfig.HandlerContext(mockConfigStore), nftMintRequest)
	assert.Error(t, err, "invalid registry address should throw an error")

	nftMintRequest = getTestSetupData()
	nftMintRequest.DepositAddress = "abc"
	handler = grpcHandler{mockConfigStore, &mockInvoiceUnpaid{}}
	_, err = handler.MintNFT(testingconfig.HandlerContext(mockConfigStore), nftMintRequest)
	assert.Error(t, err, "invalid deposit address should throw an error")
}

func getTestSetupData() *nftpb.NFTMintRequest {
	return &nftpb.NFTMintRequest{
		Identifier:      "0x12121212",
		RegistryAddress: "0xf72855759a39fb75fc7341139f5d7a3974d4da08",
		ProofFields:     []string{"gross_amount", "due_date", "currency"},
		DepositAddress:  "0xf72855759a39fb75fc7341139f5d7a3974d4da08"}
}
