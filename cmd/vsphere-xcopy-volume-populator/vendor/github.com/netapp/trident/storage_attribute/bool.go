// Copyright 2018 NetApp, Inc. All Rights Reserved.

package storageattribute

import (
	"fmt"
)

func NewBoolOffer(offer bool) Offer {
	return &boolOffer{
		Offer: offer,
	}
}

func NewBoolOfferFromOffers(offers ...Offer) Offer {
	anyTrueOffer := false

	for _, offer := range offers {
		if bOffer, ok := offer.(*boolOffer); ok {
			if bOffer.Offer {
				anyTrueOffer = true
			}
		}
	}

	// A boolOffer must hold either a true or false value.  If any of the
	// supplied offers are true, the combined result must be true.  Otherwise,
	// the supplied offers were all false, in which case the combined result
	// must be false.

	if anyTrueOffer {
		return &boolOffer{Offer: true}
	} else {
		return &boolOffer{Offer: false}
	}
}

// Matches is a boolean offer of true matches any request; a boolean offer of false
// only matches a false request.  This assumes that the requested parameter
// will be passed into the driver.
func (o *boolOffer) Matches(r Request) bool {
	br, ok := r.(*boolRequest)
	if !ok {
		return false
	}
	if o.Offer {
		return true
	}
	return br.Request == o.Offer
}

func (o *boolOffer) String() string {
	return fmt.Sprintf("{Offer:  %t}", o.Offer)
}

func (o *boolOffer) ToString() string {
	return fmt.Sprintf("%t", o.Offer)
}

func NewBoolRequest(request bool) Request {
	return &boolRequest{
		Request: request,
	}
}

func (r *boolRequest) Value() interface{} {
	return r.Request
}

func (r *boolRequest) GetType() Type {
	return boolType
}

func (r *boolRequest) String() string {
	return fmt.Sprintf("%t", r.Request)
}
