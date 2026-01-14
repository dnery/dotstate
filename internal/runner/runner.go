package runner

import (
    "bytes"
    "context"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

type CmdResult struct {
    Stdout string
    Stderr string
    Code   int
}

type Runner struct {
    Timeout time.Duration
}

func New() *Runner {
    return &Runner{Timeout: 5 * time.Minute}
}

func (r *Runner) Run(ctx context.Context, dir string, name string, args ...string) (*CmdResult, error) {
    if r.Timeout > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, r.Timeout)
        defer cancel()
    }

    cmd := exec.CommandContext(ctx, name, args...)
    if dir != "" {
        cmd.Dir = dir
    }

    var outBuf, errBuf bytes.Buffer
    cmd.Stdout = &outBuf
    cmd.Stderr = &errBuf

    err := cmd.Run()

    res := &CmdResult{
        Stdout: outBuf.String(),
        Stderr: errBuf.String(),
        Code:   0,
    }

    if err == nil {
        return res, nil
    }

    // Best-effort exit code.
    if ee, ok := err.(*exec.ExitError); ok {
        res.Code = ee.ExitCode()
    } else {
        res.Code = -1
    }

    // Include stderr in the error message for debugging.
    msg := strings.TrimSpace(res.Stderr)
    if msg == "" {
        msg = err.Error()
    }
    return res, fmt.Errorf("%s %v failed: %s", name, args, msg)
}
