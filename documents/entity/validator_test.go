// +build unit

package entity

import (
	"testing"

	"github.com/centrifuge/go-centrifuge/errors"
	"github.com/centrifuge/go-centrifuge/testingutils/commons"
	"github.com/stretchr/testify/assert"
)

func TestFieldValidator_Validate(t *testing.T) {
	fv := fieldValidator(nil)

	//  nil error
	err := fv.Validate(nil, nil)
	assert.Error(t, err)
	errs := errors.GetErrs(err)
	assert.Len(t, errs, 1, "errors length must be one")
	assert.Contains(t, errs[0].Error(), "no(nil) document provided")

	// unknown type
	err = fv.Validate(nil, &mockModel{})
	assert.Error(t, err)
	errs = errors.GetErrs(err)
	assert.Len(t, errs, 1, "errors length must be one")
	assert.Contains(t, errs[0].Error(), "document is of invalid type")

	// identity not created from the identity factory
	idFactory := new(testingcommons.MockIdentityFactory)
	entity := createEntity(t)
	idFactory.On("IdentityExists", entity.Identity).Return(false, nil).Once()
	fv = fieldValidator(idFactory)
	err = fv.Validate(nil, entity)
	assert.Error(t, err)
	idFactory.AssertExpectations(t)

	// identity created from identity factory
	idFactory = new(testingcommons.MockIdentityFactory)
	idFactory.On("IdentityExists", entity.Identity).Return(true, nil).Once()
	fv = fieldValidator(idFactory)
	err = fv.Validate(nil, entity)
	assert.NoError(t, err)
	idFactory.AssertExpectations(t)
}

func TestCreateValidator(t *testing.T) {
	cv := CreateValidator(nil)
	assert.Len(t, cv, 1)
}

func TestUpdateValidator(t *testing.T) {
	uv := UpdateValidator(nil, nil)
	assert.Len(t, uv, 2)
}
