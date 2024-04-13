// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

// Code originally from https://github.com/prometheus/common/blob/52e512c4014b/model/labelset.go#L132

package model

import (
	"fmt"
	"sort"
	"strings"
)

type LabelSet map[string]string

func (l LabelSet) String() string {
	labelNames := make([]string, 0, len(l))
	for name := range l {
		labelNames = append(labelNames, name)
	}
	sort.Strings(labelNames)
	lstrs := make([]string, 0, len(l))
	for _, name := range labelNames {
		lstrs = append(lstrs, fmt.Sprintf("%s=%q", name, l[name]))
	}
	return fmt.Sprintf("{%s}", strings.Join(lstrs, ", "))
}
