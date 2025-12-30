package builtin

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// PresetCookie represents a cookie that should be seeded into the Playwright
// context before navigating to a target domain.
type PresetCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain,omitempty"`
	Path     string `json:"path,omitempty"`
	Expires  int64  `json:"expires,omitempty"`
	HttpOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	SameSite string `json:"sameSite,omitempty"`
}

type PresetCookieJar map[string][]PresetCookie

func defaultPresetCookieJar() PresetCookieJar {
	expires := time.Now().Add(30 * 24 * time.Hour).Unix()
	return PresetCookieJar{
		".baidu.com": {
			newPresetCookie(".baidu.com", "BAIDUID", presetToken("bd", 16), expires),
			newPresetCookie(".baidu.com", "ZFY", presetToken("zfy", 18), expires),
		},
		".bing.com": {
			newPresetCookie(".bing.com", "SRCHD", presetToken("bgd", 12), expires),
			newPresetCookie(".bing.com", "SRCHHPGUSR", presetToken("hpg", 12), expires),
		},
		".bilibili.com": {
			newPresetCookie(".bilibili.com", "buvid3", presetToken("BV", 20), expires),
			newPresetCookie(".bilibili.com", "CURRENT_FNVAL", "4048", expires),
			newPresetCookie(".bilibili.com", "b_nut", presetToken("bn", 12), expires),
		},
		".douban.com": {
			newPresetCookie(".douban.com", "bid", presetToken("db", 12), expires),
			newPresetCookie(".douban.com", "dbcl2", fmt.Sprintf("\"%s\"", presetToken("db2", 12)), expires),
		},
		".douyin.com": {
			newPresetCookie(".douyin.com", "msToken", presetToken("ms", 16), expires),
			newPresetCookie(".douyin.com", "ttwid", presetToken("tt", 18), expires),
		},
		".douyu.com": {
			newPresetCookie(".douyu.com", "acf_biz", presetToken("dyb", 10), expires),
			newPresetCookie(".douyu.com", "acf_auth", presetToken("dya", 16), expires),
		},
		".google.com": {
			newPresetCookie(".google.com", "NID", presetToken("nid", 18), expires),
			newPresetCookie(".google.com", "1P_JAR", presetToken("jar", 12), expires),
		},
		".huya.com": {
			newPresetCookie(".huya.com", "udb_guid", presetToken("hy", 16), expires),
			newPresetCookie(".huya.com", "udb_deviceid", presetToken("hyd", 20), expires),
		},
		".iqiyi.com": {
			newPresetCookie(".iqiyi.com", "QC005", presetToken("iqy", 18), expires),
			newPresetCookie(".iqiyi.com", "P00001", presetToken("iqp", 18), expires),
		},
		".jd.com": {
			newPresetCookie(".jd.com", "thor", presetToken("thor", 24), expires),
			newPresetCookie(".jd.com", "pinId", presetToken("pin", 16), expires),
		},
		".kuaishou.com": {
			newPresetCookie(".kuaishou.com", "clientid", presetToken("ks", 16), expires),
			newPresetCookie(".kuaishou.com", "did", presetToken("ksd", 20), expires),
		},
		".qq.com": {
			newPresetCookie(".qq.com", "pgv_pvid", presetToken("qqp", 16), expires),
			newPresetCookie(".qq.com", "pgv_pvi", presetToken("qqv", 16), expires),
		},
		".reddit.com": {
			newPresetCookie(".reddit.com", "edgebucket", presetToken("rb", 20), expires),
			newPresetCookie(".reddit.com", "session", presetToken("rs", 24), expires),
		},
		".so.com": {
			newPresetCookie(".so.com", "QiHooGUID", presetToken("qh", 18), expires),
		},
		".sogou.com": {
			newPresetCookie(".sogou.com", "SUV", presetToken("suv", 20), expires),
			newPresetCookie(".sogou.com", "SNUID", presetToken("sn", 16), expires),
		},
		".taobao.com": {
			newPresetCookie(".taobao.com", "t", presetToken("tb", 24), expires),
			newPresetCookie(".taobao.com", "cookie2", presetToken("tb2", 24), expires),
		},
		".tmall.com": {
			newPresetCookie(".tmall.com", "cna", presetToken("tml", 22), expires),
			newPresetCookie(".tmall.com", "tfstk", presetToken("tmf", 20), expires),
		},
		".tiktok.com": {
			newPresetCookie(".tiktok.com", "tt_chain_token", presetToken("ttc", 22), expires),
			newPresetCookie(".tiktok.com", "sid_guard", presetToken("sidg", 22), expires),
		},
		".twitter.com": {
			newPresetCookie(".twitter.com", "guest_id", presetToken("twg", 18), expires),
			newPresetCookie(".twitter.com", "ct0", presetToken("twc", 24), expires),
		},
		".weixin.qq.com": {
			newPresetCookie(".weixin.qq.com", "wxuin", presetToken("wxu", 20), expires),
			newPresetCookie(".weixin.qq.com", "wxsid", presetToken("wxs", 18), expires),
		},
		".weibo.com": {
			newPresetCookie(".weibo.com", "UOR", "www.google.com", expires),
			newPresetCookie(".weibo.com", "SINAGLOBAL", presetToken("sg", 16), expires),
			newPresetCookie(".weibo.com", "SUBP", presetToken("subp", 18), expires),
		},
		".xiaohongshu.com": {
			newPresetCookie(".xiaohongshu.com", "a1", presetToken("xhs", 24), expires),
			newPresetCookie(".xiaohongshu.com", "web_session", presetToken("xhsw", 24), expires),
			newPresetCookie(".xiaohongshu.com", "webBuild", presetToken("xhsb", 16), expires),
		},
		".x.com": {
			newPresetCookie(".x.com", "guest_id", presetToken("xg", 18), expires),
			newPresetCookie(".x.com", "ct0", presetToken("xc", 24), expires),
		},
		".youtube.com": {
			newPresetCookie(".youtube.com", "VISITOR_INFO1_LIVE", presetToken("ytv", 12), expires),
			newPresetCookie(".youtube.com", "YSC", presetToken("ysc", 12), expires),
		},
		".zhihu.com": {
			newPresetCookie(".zhihu.com", "d_c0", fmt.Sprintf("\"%s\"", presetToken("dc", 14)), expires),
			newPresetCookie(".zhihu.com", "_zap", presetToken("zap", 12), expires),
			newPresetCookie(".zhihu.com", "q_c1", presetToken("qc", 16), expires),
		},
	}
}

func newPresetCookie(domain, name, value string, expires int64) PresetCookie {
	return PresetCookie{
		Name:     name,
		Value:    value,
		Domain:   domain,
		Path:     "/",
		Expires:  expires,
		Secure:   true,
		SameSite: "Lax",
	}
}

func presetToken(prefix string, bytes int) string {
	buf := make([]byte, bytes)
	_, _ = rand.Read(buf)
	hexed := hex.EncodeToString(buf)
	if prefix == "" {
		return hexed
	}
	return fmt.Sprintf("%s-%s", prefix, hexed)
}
