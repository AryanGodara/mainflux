// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mainflux/mainflux/pkg/errors"
)

const channelsEndpoint = "channels"

func (sdk mfSDK) CreateChannel(c Channel, token string) (string, errors.SDKError) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", errors.NewSDKError(err)
	}

	url := fmt.Sprintf("%s/%s", sdk.thingsURL, channelsEndpoint)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", errors.NewSDKError(err)
	}

	resp, err := sdk.sendRequest(req, token, string(CTJSON))
	if err != nil {
		return "", errors.NewSDKError(err)
	}
	defer resp.Body.Close()

	if err := errors.CheckError(resp, http.StatusCreated); err != nil {
		return "", err
	}

	id := strings.TrimPrefix(resp.Header.Get("Location"), fmt.Sprintf("/%s/", channelsEndpoint))
	return id, nil
}

func (sdk mfSDK) CreateChannels(chs []Channel, token string) ([]Channel, errors.SDKError) {
	data, err := json.Marshal(chs)
	if err != nil {
		return []Channel{}, errors.NewSDKError(err)
	}

	url := fmt.Sprintf("%s/%s/%s", sdk.thingsURL, channelsEndpoint, "bulk")

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return []Channel{}, errors.NewSDKError(err)
	}

	resp, err := sdk.sendRequest(req, token, string(CTJSON))
	if err != nil {
		return []Channel{}, errors.NewSDKError(err)
	}
	defer resp.Body.Close()

	if err := errors.CheckError(resp, http.StatusCreated); err != nil {
		return []Channel{}, err
	}

	var ccr createChannelsRes
	if err := json.NewDecoder(resp.Body).Decode(&ccr); err != nil {
		return []Channel{}, errors.NewSDKError(err)
	}

	return ccr.Channels, nil
}

func (sdk mfSDK) Channels(token string, pm PageMetadata) (ChannelsPage, errors.SDKError) {
	var url string
	var err error

	if url, err = sdk.withQueryParams(sdk.thingsURL, channelsEndpoint, pm); err != nil {
		return ChannelsPage{}, errors.NewSDKError(err)
	}

	body, sdkerr := sdk.sendRequestAndGetBodyOrError(http.MethodGet, url, nil, token, string(CTJSON), http.StatusOK)
	if sdkerr != nil {
		return ChannelsPage{}, sdkerr
	}

	var cp ChannelsPage
	if err = json.Unmarshal(body, &cp); err != nil {
		return ChannelsPage{}, errors.NewSDKError(err)
	}

	return cp, nil
}

func (sdk mfSDK) ChannelsByThing(token, thingID string, offset, limit uint64, disconn bool) (ChannelsPage, errors.SDKError) {
	url := fmt.Sprintf("%s/things/%s/channels?offset=%d&limit=%d&disconnected=%t", sdk.thingsURL, thingID, offset, limit, disconn)

	body, err := sdk.sendRequestAndGetBodyOrError(http.MethodGet, url, nil, token, string(CTJSON), http.StatusOK)
	if err != nil {
		return ChannelsPage{}, err
	}

	var cp ChannelsPage
	if err := json.Unmarshal(body, &cp); err != nil {
		return ChannelsPage{}, errors.NewSDKError(err)
	}

	return cp, nil
}

func (sdk mfSDK) Channel(id, token string) (Channel, errors.SDKError) {
	url := fmt.Sprintf("%s/%s/%s", sdk.thingsURL, channelsEndpoint, id)

	body, err := sdk.sendRequestAndGetBodyOrError(http.MethodGet, url, nil, token, string(CTJSON), http.StatusOK)
	if err != nil {
		return Channel{}, err
	}

	var c Channel
	if err := json.Unmarshal(body, &c); err != nil {
		return Channel{}, errors.NewSDKError(err)
	}

	return c, nil
}

func (sdk mfSDK) UpdateChannel(c Channel, token string) errors.SDKError {
	data, err := json.Marshal(c)
	if err != nil {
		return errors.NewSDKError(err)
	}

	url := fmt.Sprintf("%s/%s/%s", sdk.thingsURL, channelsEndpoint, c.ID)

	_, sdkerr := sdk.sendRequestAndGetBodyOrError(http.MethodPut, url, data, token, string(CTJSON), http.StatusOK)
	return sdkerr
}

func (sdk mfSDK) DeleteChannel(id, token string) errors.SDKError {
	url := fmt.Sprintf("%s/%s/%s", sdk.thingsURL, channelsEndpoint, id)

	_, err := sdk.sendRequestAndGetBodyOrError(http.MethodDelete, url, nil, token, string(CTJSON), http.StatusNoContent)
	return err
}
