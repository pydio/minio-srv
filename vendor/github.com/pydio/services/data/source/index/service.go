/*
 * Copyright 2007-2017 Abstrium <contact (at) pydio.com>
 * This file is part of Pydio.
 *
 * Pydio is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Pydio is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with Pydio.  If not, see <http://www.gnu.org/licenses/>.
 *
 * The latest code can be found at <https://pydio.com/>.
 */
package index

import (
	"context"
	"errors"

	"github.com/pydio/services/common/service"
	"github.com/pydio/services/common/service/context"

	micro "github.com/micro/go-micro"
	"github.com/pydio/services/common"
	pydio "github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/data/source/objects"
)

func NewIndexationService(ctx context.Context, datasource string) (micro.Service, error) {

	srv := service.NewService(
		micro.Name(common.SERVICE_INDEX_+datasource),
		micro.Context(servicecontext.WithDAO(ctx, NewMySQL())),
	)

	ctx = srv.Options().Context

	s3url := objects.GetS3UrlWithRetries(ctx, common.SERVICE_OBJECTS_+datasource, srv.Client(), 0)
	if s3url == "" {
		return nil, errors.New("Could not contact associated object service!")
	}

	engine := NewTreeServer(s3url, datasource)

	pydio.RegisterNodeReceiverHandler(srv.Server(), engine)
	pydio.RegisterNodeProviderHandler(srv.Server(), engine)

	return srv, nil
}
