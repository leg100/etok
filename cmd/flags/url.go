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
	u.val = val

	return nil
}

func (u *Url) String() string {
	return u.val.String()
}
