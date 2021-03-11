package flags

import "net/url"

type Url struct {
	val *url.URL
}

func (u *Url) Set(s string) error {
	val, err := url.Parse(s)
	if err != nil {
		return err
	}

	// Default to https:// if unspecified
	if val.Scheme == "" {
		val.Scheme = "https"
	}
	u.val = val

	return nil
}

func (u *Url) String() string {
	if u.val == nil {
		return ""
	}
	return u.val.String()
}

func (u *Url) Type() string {
	return "Url"
}
