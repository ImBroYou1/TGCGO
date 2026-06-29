// Package convert provides media conversion utilities for TGCGO bot.
// It is deliberately kept independent of the bot core so it can be reused.
// The implementation uses ffmpeg for video/audio conversion and libvips (via bimg)
// for image processing.

package convert

import (
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/h2non/bimg"
)

type MediaType int

const (
    Image MediaType = iota
    Video
    Audio
    Unknown
)

// DetectMediaType tries to infer the media type based on file extension.
func DetectMediaType(path string) MediaType {
    ext := strings.ToLower(filepath.Ext(path))
    switch ext {
    case ".jpg", ".jpeg", ".png", ".webp", ".avif", ".gif", ".bmp", ".tiff":
        return Image
    case ".mp4", ".mkv", ".webm", ".avi", ".mov", ".flv", ".gif":
        return Video
    case ".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a":
        return Audio
    default:
        return Unknown
    }
}

// ImageOpts defines optional parameters for image conversion.
type ImageOpts struct {
    Width   int    // 0 means keep original width
    Height  int    // 0 means keep original height
    Quality int    // 0-100, 0 uses default
    Format  string // target format: "png", "jpeg", "webp", "avif"
}

// ConvertImage converts an image using libvips via bimg.
func ConvertImage(src, dst string, opts ImageOpts) error {
    buffer, err := bimg.Read(src)
    if err != nil {
        return fmt.Errorf("read source image: %w", err)
    }
    image := bimg.NewImage(buffer)
    var newImage []byte
    // Build options
    bopts := bimg.Options{}
    if opts.Width > 0 {
        bopts.Width = opts.Width
    }
    if opts.Height > 0 {
        bopts.Height = opts.Height
    }
    if opts.Quality > 0 {
        bopts.Quality = opts.Quality
    }
    switch strings.ToLower(opts.Format) {
    case "png":
        bopts.Type = bimg.PNG
    case "jpeg", "jpg":
        bopts.Type = bimg.JPEG
    case "webp":
        bopts.Type = bimg.WEBP
    case "avif":
        bopts.Type = bimg.AVIF
    default:
        // keep original format if unspecified
    }
    newImage, err = image.Process(bopts)
    if err != nil {
        return fmt.Errorf("process image: %w", err)
    }
    if err = bimg.Write(dst, newImage); err != nil {
        return fmt.Errorf("write output image: %w", err)
    }
    return nil
}

// VideoOpts holds ffmpeg parameters for video conversion.
type VideoOpts struct {
    Format string // e.g., "mp4", "mkv", "webm", "gif"
    // Additional ffmpeg args can be extended later.
}

// ConvertVideo uses ffmpeg to convert video/audio files.
func ConvertVideo(src, dst string, opts VideoOpts) error {
    if opts.Format == "" {
        return errors.New("target video format not specified")
    }
    // Ensure ffmpeg exists; let command fail with clear error if not.
    args := []string{"-i", src, "-y", dst}
    cmd := exec.Command("ffmpeg", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("ffmpeg conversion failed: %w", err)
    }
    return nil
}

// AudioOpts holds ffmpeg parameters for audio conversion.
type AudioOpts struct {
    Format string // e.g., "mp3", "wav", "ogg", "flac"
}

// ConvertAudio uses ffmpeg to convert audio files.
func ConvertAudio(src, dst string, opts AudioOpts) error {
    if opts.Format == "" {
        return errors.New("target audio format not specified")
    }
    args := []string{"-i", src, "-y", dst}
    cmd := exec.Command("ffmpeg", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("ffmpeg audio conversion failed: %w", err)
    }
    return nil
}

// Convert is a high‑level wrapper that detects the media type and delegates.
func Convert(srcPath, dstPath string, imageOpts *ImageOpts, videoOpts *VideoOpts, audioOpts *AudioOpts) error {
    if _, err := os.Stat(srcPath); err != nil {
        return fmt.Errorf("source file not found: %w", err)
    }
    mt := DetectMediaType(srcPath)
    switch mt {
    case Image:
        if imageOpts == nil {
            imageOpts = &ImageOpts{}
        }
        if imageOpts.Format == "" {
            ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(dstPath)), ".")
            imageOpts.Format = ext
        }
        return ConvertImage(srcPath, dstPath, *imageOpts)
    case Video:
        if videoOpts == nil {
            videoOpts = &VideoOpts{}
        }
        if videoOpts.Format == "" {
            videoOpts.Format = strings.TrimPrefix(strings.ToLower(filepath.Ext(dstPath)), ".")
        }
        return ConvertVideo(srcPath, dstPath, *videoOpts)
    case Audio:
        if audioOpts == nil {
            audioOpts = &AudioOpts{}
        }
        if audioOpts.Format == "" {
            audioOpts.Format = strings.TrimPrefix(strings.ToLower(filepath.Ext(dstPath)), ".")
        }
        return ConvertAudio(srcPath, dstPath, *audioOpts)
    default:
        return errors.New("unsupported media type")
    }
}
