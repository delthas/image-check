package imagecheck

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/ulikunitz/xz/lzma"
)

func wrap(f string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(f, err)
}

func checkJpeg(f *os.File) error {
	_, err := f.Seek(0, io.SeekStart)
	if err != nil {
		return wrap("seeking to file start: %v", err)
	}
	bf := bufio.NewReader(f)
	markerStart := false
	for {
		b, err := bf.ReadByte()
		if err != nil {
			return wrap("reading marker start or entropy-coded data: %v", err)
		}
		if b == 0xFF {
			markerStart = true
			continue
		}
		if b == 0x00 {
			markerStart = false
			continue
		}
		if markerStart {
			if b == 0xD9 {
				return nil
			}
			if b >= 0xD0 && b < 0xD9 {
				markerStart = false
				continue
			}
			buf := make([]byte, 2)
			_, err := io.ReadFull(bf, buf)
			if err != nil {
				return wrap("reading marker size: %v", err)
			}
			size := int(buf[0])<<8 | int(buf[1])
			_, err = bf.Discard(size)
			if err != nil {
				return wrap("skipping marker data: %v", err)
			}
			markerStart = false
		}
	}
}

func checkPng(f *os.File) error {
	endSignature := []byte{0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82}
	iend := []byte{0x49, 0x45, 0x4E, 0x44}
	_, err := f.Seek(int64(-len(endSignature)), io.SeekEnd)
	if err != nil {
		return wrap("seeking to trailing IEND block: %v", err)
	}
	end := make([]byte, len(endSignature))
	_, err = io.ReadFull(f, end)
	if err != nil {
		return wrap("reading trailing IEND block: %v", err)
	}
	if !bytes.Equal(end, endSignature) {
		return errors.New("file does not end with IEND block")
	}
	_, err = f.Seek(8, io.SeekStart)
	if err != nil {
		return wrap("seeking to first block: %v", err)
	}
	bf := bufio.NewReader(f)
	buf := make([]byte, 4)
	for {
		_, err := io.ReadFull(bf, buf)
		if err != nil {
			return wrap("reading block size: %v", err)
		}
		length := int(binary.BigEndian.Uint32(buf))
		_, err = io.ReadFull(bf, buf)
		if err != nil {
			return wrap("reading block type: %v", err)
		}
		if bytes.Equal(buf, iend) {
			// IEND chunk
			_, err := bf.Discard(4)
			if err != nil {
				return wrap("skipping IEND block: %v", err)
			}
			return nil
		}
		_, err = bf.Discard(4 + length)
		if err != nil {
			return wrap("skipping block data: %v", err)
		}
	}
}

func checkGif(f *os.File) error {
	_, err := f.Seek(-1, io.SeekEnd)
	if err != nil {
		return wrap("seeking to GIF trailer: %v", err)
	}
	end := make([]byte, 1)
	_, err = io.ReadFull(f, end)
	if err != nil {
		return wrap("reading GIF trailer: %v", err)
	}
	if end[0] != 0x3B {
		return errors.New("file does not end with GIF trailer")
	}
	_, err = f.Seek(10, io.SeekStart)
	if err != nil {
		return wrap("seeking to header bitflag: %v", err)
	}
	bf := bufio.NewReader(f)
	bitflag := make([]byte, 1)
	_, err = io.ReadFull(bf, bitflag)
	if err != nil {
		return wrap("reading header bitflag: %v", err)
	}
	skip := 2
	if bitflag[0]&0b10000000 != 0 {
		skip += 3 * (1 << (1 + int(bitflag[0]&0b111)))
	}
	_, err = bf.Discard(skip)
	if err != nil {
		return wrap("skipping header: %v", err)
	}
	for {
		blockType, err := bf.ReadByte()
		if err != nil {
			return wrap("reading block type: %v", err)
		}
		if blockType == 0x3B {
			return nil
		}
		var skip int
		if blockType == 0x2C {
			_, err = bf.Discard(8)
			if err != nil {
				return wrap("skipping image block data: %v", err)
			}
			skip = 1
			if bitflag[0]&0b10000000 != 0 {
				skip += 3 * (1 << (1 + int(bitflag[0]&0b1111)))
			}
		} else {
			skip = 1
		}
		_, err = bf.Discard(skip)
		if err != nil {
			return wrap("skipping block data: %v", err)
		}
		for {
			size, err := bf.ReadByte()
			if err != nil {
				return wrap("reading sub-block size: %v", err)
			}
			if size == 0 {
				break
			}
			_, err = bf.Discard(int(size))
			if err != nil {
				return wrap("skipping sub-block data: %v", err)
			}
		}
	}
}

func checkSwf(f *os.File) error {
	_, err := f.Seek(0, io.SeekStart)
	if err != nil {
		return wrap("seeking to start of file: %v", err)
	}
	buf := make([]byte, 4)
	_, err = io.ReadFull(f, buf[:1])
	if err != nil {
		return wrap("reading SWF file signature: %v", err)
	}
	compressionType := buf[0]
	_, err = f.Seek(4, io.SeekStart)
	if err != nil {
		return wrap("seeking to file size header: %v", err)
	}
	_, err = io.ReadFull(f, buf)
	if err != nil {
		return wrap("reading file size header: %v", err)
	}
	size := int64(binary.LittleEndian.Uint32(buf))
	if compressionType == 0x46 {
		total, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return wrap("seeking to file end: %v", err)
		}
		if total < size {
			return errors.New("file length is less than file size header")
		}
		return nil
	}
	if compressionType == 0x43 {
		zr, err := zlib.NewReader(f)
		if err != nil {
			return wrap("parsing ZLIB compressed data signature: %v", err)
		}
		n, err := io.Copy(ioutil.Discard, zr)
		if err != nil {
			return wrap("decompressing ZLIB compressed data: %v", err)
		}
		n += 8
		if n < size {
			return errors.New("ZLIB-decompressed data length is less than file size header")
		}
		return nil
	}
	if compressionType == 0x5A {
		lr, err := lzma.NewReader(f)
		if err != nil {
			return wrap("parsing LZMA compressed data signature: %v", err)
		}
		n, err := io.Copy(ioutil.Discard, lr)
		if err != nil {
			return wrap("decompressing LZMA compressed data: %v", err)
		}
		n += 8
		if n < size {
			return errors.New("LZMA-compressed data length is less than file size header")
		}
		return nil
	}
	return errors.New("file has unknown compression type")
}

func checkFile(f *os.File) error {
	signature := make([]byte, 8)
	_, err := io.ReadFull(f, signature)
	if err != nil {
		return wrap("reading file signature: %v", err)
	}
	pngSignature := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	gifSignature := []byte{0x47, 0x49, 0x46, 0x38}
	jpegSignature := []byte{0xFF, 0xD8}
	swfSignature := []byte{0x57, 0x53}
	if bytes.HasPrefix(signature, pngSignature) {
		return wrap("PNG file: %v", checkPng(f))
	}
	if bytes.HasPrefix(signature, gifSignature) {
		return wrap("GIF file: %v", checkGif(f))
	}
	if bytes.HasPrefix(signature, jpegSignature) {
		return wrap("JPEG file: %v", checkJpeg(f))
	}
	if bytes.HasPrefix(signature[1:], swfSignature) {
		return wrap("SWF file: %v", checkSwf(f))
	}
	return errors.New("unknown image file format")
}

func Check(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return checkFile(f)
}
