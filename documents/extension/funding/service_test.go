// +build unit

package funding

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/centrifuge/centrifuge-protobufs/gen/go/coredocument"
	"github.com/centrifuge/go-centrifuge/anchors"
	"github.com/centrifuge/go-centrifuge/bootstrap"
	"github.com/centrifuge/go-centrifuge/bootstrap/bootstrappers/testlogging"
	"github.com/centrifuge/go-centrifuge/config"
	"github.com/centrifuge/go-centrifuge/config/configstore"
	"github.com/centrifuge/go-centrifuge/contextutil"
	"github.com/centrifuge/go-centrifuge/documents"
	"github.com/centrifuge/go-centrifuge/documents/invoice"
	"github.com/centrifuge/go-centrifuge/ethereum"
	"github.com/centrifuge/go-centrifuge/identity/ideth"
	"github.com/centrifuge/go-centrifuge/jobs"
	"github.com/centrifuge/go-centrifuge/p2p"
	clientfundingpb "github.com/centrifuge/go-centrifuge/protobufs/gen/go/funding"
	"github.com/centrifuge/go-centrifuge/queue"
	"github.com/centrifuge/go-centrifuge/storage/leveldb"
	"github.com/centrifuge/go-centrifuge/testingutils/documents"
	"github.com/centrifuge/go-centrifuge/testingutils/identity"
	"github.com/centrifuge/go-centrifuge/testingutils/testingjobs"
	"github.com/centrifuge/go-centrifuge/utils"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var ctx = map[string]interface{}{}
var cfg config.Configuration

var (
	did = testingidentity.GenerateRandomDID()
)

func TestMain(m *testing.M) {
	ethClient := &ethereum.MockEthClient{}
	ethClient.On("GetEthClient").Return(nil)
	ctx[ethereum.BootstrappedEthereumClient] = ethClient
	jobMan := &testingjobs.MockJobManager{}
	ctx[jobs.BootstrappedService] = jobMan
	done := make(chan bool)
	jobMan.On("ExecuteWithinJob", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(jobs.NilJobID(), done, nil)
	ctx[bootstrap.BootstrappedInvoiceUnpaid] = new(testingdocuments.MockRegistry)
	ibootstrappers := []bootstrap.TestBootstrapper{
		&testlogging.TestLoggingBootstrapper{},
		&config.Bootstrapper{},
		&leveldb.Bootstrapper{},
		&queue.Bootstrapper{},
		&ideth.Bootstrapper{},
		&configstore.Bootstrapper{},
		anchors.Bootstrapper{},
		documents.Bootstrapper{},
		p2p.Bootstrapper{},
		documents.PostBootstrapper{},
		// &Bootstrapper{}, // todo add own bootstrapper
		&queue.Starter{},
	}
	bootstrap.RunTestBootstrappers(ibootstrappers, ctx)
	cfg = ctx[bootstrap.BootstrappedConfig].(config.Configuration)
	cfg.Set("identityId", did.String())
	configService = ctx[config.BootstrappedConfigStorage].(config.Service)
	result := m.Run()
	bootstrap.RunTestTeardown(ibootstrappers)
	os.Exit(result)
}

func TestGenerateKey(t *testing.T) {
	assert.Equal(t, "funding_agreement[1].days", generateLabel(fundingFieldKey, "1", "days"))
	assert.Equal(t, "funding_agreement[0].", generateLabel(fundingFieldKey, "0", ""))

}

func TestCreateAttributesList(t *testing.T) {
	testingdocuments.CreateInvoicePayload()
	inv := &invoice.Invoice{}
	err := inv.InitInvoiceInput(testingdocuments.CreateInvoicePayload(), testingidentity.GenerateRandomDID())
	assert.NoError(t, err)

	data := createTestData()

	attributes, err := createAttributesList(inv, data)
	assert.NoError(t, err)

	assert.Equal(t, 11, len(attributes))

	for _, attribute := range attributes {
		if attribute.KeyLabel == "funding_agreement[0].currency" {
			assert.Equal(t, "eur", attribute.Value.Str)
			break
		}

		// apr was not set
		assert.NotEqual(t, "funding_agreement[0].apr", attribute.KeyLabel)
	}
}

func TestDeriveFromPayload(t *testing.T) {
	testingdocuments.CreateInvoicePayload()
	inv := &invoice.Invoice{}
	err := inv.InitInvoiceInput(testingdocuments.CreateInvoicePayload(), testingidentity.GenerateRandomDID())
	assert.NoError(t, err)

	docSrv := &testingdocuments.MockService{}
	docSrv.On("GetCurrentVersion", mock.Anything, mock.Anything).Return(inv, nil)
	srv := DefaultService(docSrv, nil)

	payload := createTestPayload()

	for i := 0; i < 10; i++ {
		model, err := srv.DeriveFromPayload(context.Background(), payload, utils.RandomSlice(32))
		assert.NoError(t, err)
		label := fmt.Sprintf("funding_agreement[%d].currency", i)
		key, err := documents.AttrKeyFromLabel(label)
		assert.NoError(t, err)

		attr, err := model.GetAttribute(key)
		assert.NoError(t, err)
		assert.Equal(t, "eur", attr.Value.Str)

	}

}

func TestDeriveFundingResponse(t *testing.T) {
	testingdocuments.CreateInvoicePayload()
	inv := &invoice.Invoice{}
	err := inv.InitInvoiceInput(testingdocuments.CreateInvoicePayload(), testingidentity.GenerateRandomDID())
	assert.NoError(t, err)

	docSrv := &testingdocuments.MockService{}
	docSrv.On("GetCurrentVersion", mock.Anything, mock.Anything).Return(inv, nil)
	srv := DefaultService(docSrv, nil)

	for i := 0; i < 10; i++ {
		payload := createTestPayload()
		model, err := srv.DeriveFromPayload(context.Background(), payload, utils.RandomSlice(32))
		assert.NoError(t, err)

		response, err := srv.DeriveFundingResponse(model, payload.Data.FundingId)
		assert.NoError(t, err)
		checkResponse(t, payload, response.Data)
	}

}

func TestDeriveFundingListResponse(t *testing.T) {
	testingdocuments.CreateInvoicePayload()
	inv := &invoice.Invoice{}
	err := inv.InitInvoiceInput(testingdocuments.CreateInvoicePayload(), testingidentity.GenerateRandomDID())
	assert.NoError(t, err)

	docSrv := &testingdocuments.MockService{}
	docSrv.On("GetCurrentVersion", mock.Anything, mock.Anything).Return(inv, nil)
	srv := DefaultService(docSrv, nil)

	var model documents.Model
	var payloads []*clientfundingpb.FundingCreatePayload
	for i := 0; i < 10; i++ {
		p := createTestPayload()
		payloads = append(payloads, p)
		model, err = srv.DeriveFromPayload(context.Background(), p, utils.RandomSlice(32))
		assert.NoError(t, err)

	}

	response, err := srv.DeriveFundingListResponse(model)
	assert.NoError(t, err)
	assert.Equal(t, 10, len(response.List))

	for i := 0; i < 10; i++ {
		checkResponse(t, payloads[i], response.List[i])

	}

}

type mockAccount struct {
	config.Account
	mock.Mock
}

func (m *mockAccount) SignMsg(msg []byte) (*coredocumentpb.Signature, error) {
	args := m.Called(msg)
	sig, _ := args.Get(0).(*coredocumentpb.Signature)
	return sig, args.Error(1)
}

func (m *mockAccount) GetIdentityID() ([]byte, error) {
	args := m.Called()
	sig, _ := args.Get(0).([]byte)
	return sig, args.Error(1)
}

func setupFundingsForTesting(t *testing.T, fundingAmount int) (Service, documents.Model, string) {
	testingdocuments.CreateInvoicePayload()
	inv := &invoice.Invoice{}
	err := inv.InitInvoiceInput(testingdocuments.CreateInvoicePayload(), testingidentity.GenerateRandomDID())
	assert.NoError(t, err)

	docSrv := &testingdocuments.MockService{}
	docSrv.On("GetCurrentVersion", mock.Anything, mock.Anything).Return(inv, nil)
	srv := DefaultService(docSrv, nil)

	var model documents.Model
	var payloads []*clientfundingpb.FundingCreatePayload

	var lastFundingId string

	// create a list of fundings
	for i := 0; i < fundingAmount; i++ {
		p := createTestPayload()
		payloads = append(payloads, p)
		model, err = srv.DeriveFromPayload(context.Background(), p, utils.RandomSlice(32))
		assert.NoError(t, err)
		lastFundingId = p.Data.FundingId

	}

	return srv, model, lastFundingId
}

func TestService_Sign(t *testing.T) {
	fundingAmount := 5
	srv, model, lastFundingId := setupFundingsForTesting(t, fundingAmount)

	// add signature
	acc := &mockAccount{}
	acc.On("GetIdentityID").Return(utils.RandomSlice(20), nil)
	// success
	signature := utils.RandomSlice(32)
	acc.On("SignMsg", mock.Anything).Return(&coredocumentpb.Signature{Signature: signature}, nil)
	ctx, err := contextutil.New(context.Background(), acc)
	assert.NoError(t, err)

	for i := 0; i < 5; i++ {
		model, err = srv.Sign(ctx, lastFundingId, utils.RandomSlice(32))
		assert.NoError(t, err)
		// signature should exist
		label := fmt.Sprintf("funding_agreement[%d].signatures[%d]", fundingAmount-1, i)
		key, err := documents.AttrKeyFromLabel(label)
		assert.NoError(t, err)
		attr, err := model.GetAttribute(key)
		assert.NoError(t, err)
		assert.Equal(t, documents.AttrSigned, attr.Value.Type)
		assert.Equal(t, signature, attr.Value.Signed.Signature)

		// array idx should exist
		label = fmt.Sprintf("funding_agreement[%d].signatures", fundingAmount-1)
		key, err = documents.AttrKeyFromLabel(label)
		assert.NoError(t, err)
		attr, err = model.GetAttribute(key)
		assert.Equal(t, fmt.Sprintf("%d", i), attr.Value.Int256.String())
		assert.NoError(t, err)
	}

	// funding id not exists
	model, err = srv.Sign(ctx, hexutil.Encode(utils.RandomSlice(32)), utils.RandomSlice(32))
	assert.Error(t, err)
}

func TestService_DeriveFromUpdatePayload(t *testing.T) {
	testingdocuments.CreateInvoicePayload()
	inv := &invoice.Invoice{}
	inv.InitInvoiceInput(testingdocuments.CreateInvoicePayload(), testingidentity.GenerateRandomDID())

	docSrv := &testingdocuments.MockService{}
	docSrv.On("GetCurrentVersion", mock.Anything, mock.Anything).Return(inv, nil)
	srv := DefaultService(docSrv, nil)

	var model documents.Model
	var err error

	p := createTestPayload()
	model, err = srv.DeriveFromPayload(context.Background(), p, utils.RandomSlice(32))
	assert.NoError(t, err)

	// update
	docSrv.On("GetCurrentVersion", mock.Anything, mock.Anything).Return(model, nil)
	p2 := &clientfundingpb.FundingUpdatePayload{Data: createTestClientData(), Identifier: hexutil.Encode(utils.RandomSlice(32)), FundingId: p.Data.FundingId}
	p2.Data.Currency = ""
	p2.Data.Fee = "13.37"

	model, err = srv.DeriveFromUpdatePayload(context.Background(), p2, utils.RandomSlice(32))
	assert.NoError(t, err)

	response, err := srv.DeriveFundingListResponse(model)
	assert.Equal(t, 1, len(response.List))
	assert.Equal(t, p2.Data.Fee, response.List[0].Fee)

	// fee was not set in the update old fee field should not exist
	assert.NotEqual(t, p.Data.Fee, response.List[0].Fee)

	// non existing funding id
	p3 := &clientfundingpb.FundingUpdatePayload{Data: createTestClientData(), Identifier: hexutil.Encode(utils.RandomSlice(32)), FundingId: hexutil.Encode(utils.RandomSlice(32))}
	model, err = srv.DeriveFromUpdatePayload(context.Background(), p3, utils.RandomSlice(32))
	assert.Error(t, err)
	assert.Contains(t, err, ErrFundingNotFound)
}

func createTestClientData() *clientfundingpb.FundingData {
	fundingId := newFundingID()
	return &clientfundingpb.FundingData{
		FundingId:             fundingId,
		Currency:              "eur",
		Days:                  "90",
		Amount:                "1000",
		RepaymentAmount:       "1200.12",
		Fee:                   "10",
		NftAddress:            hexutil.Encode(utils.RandomSlice(32)),
		RepaymentDueDate:      time.Now().UTC().Format(time.RFC3339),
		RepaymentOccurredDate: time.Now().UTC().Format(time.RFC3339),
		PaymentDetailsId:      hexutil.Encode(utils.RandomSlice(32)),
	}
}

func createTestData() Data {
	fundingId := newFundingID()
	return Data{
		FundingId:             fundingId,
		Currency:              "eur",
		Days:                  "90",
		Amount:                "1000",
		RepaymentAmount:       "1200.12",
		Fee:                   "10",
		NftAddress:            hexutil.Encode(utils.RandomSlice(32)),
		RepaymentDueDate:      time.Now().UTC().Format(time.RFC3339),
		RepaymentOccurredDate: time.Now().UTC().Format(time.RFC3339),
		PaymentDetailsId:      hexutil.Encode(utils.RandomSlice(32)),
	}
}

func createTestPayload() *clientfundingpb.FundingCreatePayload {
	return &clientfundingpb.FundingCreatePayload{Data: createTestClientData()}
}

func checkResponse(t *testing.T, payload *clientfundingpb.FundingCreatePayload, response *clientfundingpb.FundingData) {
	assert.Equal(t, payload.Data.FundingId, response.FundingId)
	assert.Equal(t, payload.Data.Currency, response.Currency)
	assert.Equal(t, payload.Data.Days, response.Days)
	assert.Equal(t, payload.Data.Amount, response.Amount)
	assert.Equal(t, payload.Data.RepaymentDueDate, response.RepaymentDueDate)
}
