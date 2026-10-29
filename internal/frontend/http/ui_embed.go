// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package http

import _ "embed" // for llms.txt

//go:embed templates/llms.txt
var llmsTxtContent []byte

func getLLMsTxt() []byte { return llmsTxtContent }
