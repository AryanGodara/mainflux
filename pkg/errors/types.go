// Copyright (c) Mainflux
// SPDX-License-Identifier: Apache-2.0

package errors

var (
	// ErrAuthentication indicates failure occurred while authenticating the entity.
	ErrAuthentication = New("failed to perform authentication over the entity")

	// ErrAuthorization indicates failure occurred while authorizing the entity.
	ErrAuthorization = New("failed to perform authorization over the entity")

	// ErrUnsupportedContentType indicates unacceptable or lack of Content-Type.
	ErrUnsupportedContentType = New("unsupported content type")

	// ErrNotFound indicates a non-existent entity request.
	ErrNotFound = New("entity not found")

	// ErrConflict indicates that entity already exists.
	ErrConflict = New("entity already exists")

	// ErrCreateEntity indicates error in creating entity or entities.
	ErrCreateEntity = New("failed to create entity in the db")

	// ErrViewEntity indicates error in viewing entity or entities.
	ErrViewEntity = New("view entity failed")

	// ErrUpdateEntity indicates error in updating entity or entities.
	ErrUpdateEntity = New("update entity failed")

	// ErrRemoveEntity indicates error in removing entity.
	ErrRemoveEntity = New("failed to remove entity")

	// ErrScanMetadata indicates problem with metadata in db.
	ErrScanMetadata = New("failed to scan metadata in db")

	// ErrWrongSecret indicates a wrong secret was provided.
	ErrWrongSecret = New("wrong secret")
)
