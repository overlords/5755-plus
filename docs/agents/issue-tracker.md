# Issue tracker:GitHub

本仓库的 issues 和 PRD 以 GitHub issue 形式记录,所有操作使用 `gh` CLI。

> **前置条件**:本仓库当前没有 git remote。使用前需先关联 GitHub 仓库(`gh repo create` 或 `git remote add origin <url>`),否则 `gh issue` 命令无法定位仓库。

## 约定

- **创建 issue**:`gh issue create --title "..." --body "..."`;多行 body 使用 heredoc。
- **读取 issue**:`gh issue view <number> --comments`,可用 `jq` 过滤评论并获取标签。
- **列出 issues**:`gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'`,按需加 `--label`、`--state` 过滤。
- **评论**:`gh issue comment <number> --body "..."`
- **加/去标签**:`gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **关闭**:`gh issue close <number> --comment "..."`

仓库由 `git remote -v` 推断——在 clone 内运行时 `gh` 自动处理。

## 当 skill 说"发布到 issue tracker"时

创建一个 GitHub issue。

## 当 skill 说"获取相关 ticket"时

运行 `gh issue view <number> --comments`。
