/*
 * Copyright © 2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @copyright 	2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @license 	Apache-2.0
 */
package herodot

import (
	"fmt"
	"reflect"

	"github.com/pkg/errors"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

// getErrorTrace is a helper that returns an error's trace
func getErrorTrace(err error) string {
	if e, ok := err.(stackTracer); ok {
		return fmt.Sprintf("Stack trace: %+v", e.StackTrace())
	}

	return fmt.Sprintf("Stack trace could not be recovered from error type %s", reflect.TypeOf(err))
}
