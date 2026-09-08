package team

import "encoding/json"

func unmarshalSubOrgs(data []byte, out *[]SubOrgRef) error {
	if len(data) == 0 {
		*out = nil
		return nil
	}
	return json.Unmarshal(data, out)
}
