// Package apksig 实现 APK Signing Block 自定义条目的读写(免重签渠道写入,M4 渠道三件套)。
//
// 块格式契约(与 Android SDK 读取端共享,跨端黄金向量钉死):
//   - 渠道条目 ID:0x71777777(兼容读取 0x57550001);
//   - 条目值:UTF-8 渠道标识符原串(已按 01 §6 归一化)。
//
// APK Signing Block 布局(v2/v3 签名方案,位于 ZIP 中央目录之前):
//   uint64 size(不含本字段)| pairs... | uint64 size(同前)| magic "APK Sig Block 42"
//   每个 pair:uint64 len | uint32 ID | value([len-4]byte)
// 写入只增/替换自定义 ID 条目,不触碰 v2/v3 签名条目,因此 apksigner verify 仍通过。
package apksig

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

const (
	// ChannelBlockID 渠道条目主 ID(写入用;读取端兼容两 ID)。
	ChannelBlockID uint32 = 0x71777777
	// ChannelBlockIDAlt 兼容读取 ID。
	ChannelBlockIDAlt uint32 = 0x57550001

	magic       = "APK Sig Block 42"
	eocdMinSize = 22
)

// 关键偏移:EOCD 中央目录偏移字段位置(EOCD 起点 +16)。
const eocdCDOffsetPos = 16

type apkLayout struct {
	data        []byte
	sigStart    int64 // Signing Block 起点(含前置 size 字段)
	sigEnd      int64 // Signing Block 终点(= 中央目录起点)
	cdOffset    int64 // 中央目录偏移(EOCD 记录值)
	eocdPos     int64
	hasSigBlock bool
}

func parse(data []byte) (*apkLayout, error) {
	// 定位 EOCD(无注释时在文件尾;保守向前扫描 64KB+22)
	scanFrom := len(data) - eocdMinSize - 65535
	if scanFrom < 0 {
		scanFrom = 0
	}
	eocd := -1
	for i := len(data) - eocdMinSize; i >= scanFrom; i-- {
		if binary.LittleEndian.Uint32(data[i:]) == 0x06054b50 {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		return nil, errors.New("找不到 ZIP EOCD,非法 APK")
	}
	cd := int64(binary.LittleEndian.Uint32(data[eocd+eocdCDOffsetPos:]))
	l := &apkLayout{data: data, cdOffset: cd, eocdPos: int64(eocd), sigEnd: cd}

	// Signing Block 紧贴中央目录之前:尾部 24 字节 = uint64 size + magic
	if cd >= int64(24) && string(data[cd-16:cd]) == magic {
		size := int64(binary.LittleEndian.Uint64(data[cd-24 : cd-16]))
		start := cd - size - 8 // size 字段不含自身
		if start < 0 || int64(binary.LittleEndian.Uint64(data[start:])) != size {
			return nil, errors.New("Signing Block 前后 size 不一致")
		}
		l.sigStart = start
		l.hasSigBlock = true
	} else {
		return nil, errors.New("APK 无 v2/v3 Signing Block(请先用 apksigner 签名)")
	}
	return l, nil
}

// pairs 解析块内全部条目(ID → value)。保持顺序。
type pair struct {
	id    uint32
	value []byte
}

func (l *apkLayout) pairs() ([]pair, error) {
	size := int64(binary.LittleEndian.Uint64(l.data[l.sigStart:]))
	payload := l.data[l.sigStart+8 : l.sigStart+8+size-24] // 去掉尾部 size+magic
	var out []pair
	for off := int64(0); off < int64(len(payload)); {
		if off+12 > int64(len(payload)) {
			return nil, errors.New("Signing Block 条目截断")
		}
		plen := int64(binary.LittleEndian.Uint64(payload[off:]))
		id := binary.LittleEndian.Uint32(payload[off+8:])
		if off+8+plen > int64(len(payload)) || plen < 4 {
			return nil, errors.New("Signing Block 条目长度非法")
		}
		val := payload[off+12 : off+8+plen]
		out = append(out, pair{id: id, value: append([]byte(nil), val...)})
		off += 8 + plen
	}
	return out, nil
}

// ReadChannel 从 APK 读渠道条目(主 ID 优先,兼容 ID 兜底)。无则返回 ""。
func ReadChannel(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	l, err := parse(data)
	if err != nil {
		return "", err
	}
	ps, err := l.pairs()
	if err != nil {
		return "", err
	}
	var alt string
	for _, p := range ps {
		if p.id == ChannelBlockID {
			return string(p.value), nil
		}
		if p.id == ChannelBlockIDAlt {
			alt = string(p.value)
		}
	}
	return alt, nil
}

// WriteChannel 写入(替换同 ID)渠道条目并产出新 APK 字节。免重签:不动签名条目。
func WriteChannel(data []byte, channel string) ([]byte, error) {
	l, err := parse(data)
	if err != nil {
		return nil, err
	}
	ps, err := l.pairs()
	if err != nil {
		return nil, err
	}
	// 替换/追加渠道条目(幂等:同 ID 仅保留一条)
	kept := ps[:0]
	for _, p := range ps {
		if p.id != ChannelBlockID && p.id != ChannelBlockIDAlt {
			kept = append(kept, p)
		}
	}
	kept = append(kept, pair{id: ChannelBlockID, value: []byte(channel)})

	// 重组块
	var payload bytes.Buffer
	for _, p := range kept {
		plen := uint64(4 + len(p.value))
		_ = binary.Write(&payload, binary.LittleEndian, plen)
		_ = binary.Write(&payload, binary.LittleEndian, p.id)
		payload.Write(p.value)
	}
	blockSize := uint64(payload.Len() + 24) // payload + 尾 size + magic
	var block bytes.Buffer
	_ = binary.Write(&block, binary.LittleEndian, blockSize)
	block.Write(payload.Bytes())
	_ = binary.Write(&block, binary.LittleEndian, blockSize)
	block.WriteString(magic)

	// 拼接新 APK:Block 前内容 + 新 Block + 中央目录及之后
	var out bytes.Buffer
	out.Write(data[:l.sigStart])
	out.Write(block.Bytes())
	out.Write(data[l.cdOffset:])

	// 修正 EOCD 的中央目录偏移
	newCD := l.sigStart + int64(block.Len())
	delta := newCD - l.cdOffset
	result := out.Bytes()
	newEOCD := l.eocdPos + delta
	binary.LittleEndian.PutUint32(result[newEOCD+eocdCDOffsetPos:], uint32(newCD))
	return result, nil
}

// WriteChannelFile 文件级封装。
func WriteChannelFile(in, outPath, channel string) error {
	data, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	res, err := WriteChannel(data, channel)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, res, 0o644)
}

// 编译期防误用提示:本包仅供 channel-pack CLI 与测试使用,不得链入 m5755-server 运行时。
var _ = fmt.Sprintf
