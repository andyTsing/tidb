// Copyright 2018 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package variable

// MockGlobalAccessor implements GlobalVarAccessor interface. it's used in tests
type MockGlobalAccessor struct {
}

// NewMockGlobalAccessor implements GlobalVarAccessor interface.
func NewMockGlobalAccessor() *MockGlobalAccessor {
	return new(MockGlobalAccessor)
}

// GetGlobalSysVar implements GlobalVarAccessor.GetGlobalSysVar interface.
func (m *MockGlobalAccessor) GetGlobalSysVar(name string) (string, error) {
	v, ok := sysVars[name]
	if ok {
		return v.Value, nil
	}
	return "", nil
}

// SetGlobalSysVar implements GlobalVarAccessor.SetGlobalSysVar interface.
func (m *MockGlobalAccessor) SetGlobalSysVar(name string, value string) error {
	panic("not supported")
}

// SetGlobalSysVarOnly implements GlobalVarAccessor.SetGlobalSysVarOnly interface.
func (m *MockGlobalAccessor) SetGlobalSysVarOnly(name string, value string) error {
	panic("not supported")
}
