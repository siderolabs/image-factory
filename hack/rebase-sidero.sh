#!/bin/sh

### Rebase this fork's patch series onto an upstream siderolabs/image-factory tag.
### Arguments:
###   $1 - upstream tag (e.g., v1.8.0), --continue to resume after a manual conflict
###        resolution, or --abort to undo the rebase and delete the release branch.
###
### Creates the branch versions/<ver> from the current branch and rebases every commit since
### the merge base with the tag onto the tag, so each release carries the same linear patch
### series. Conflicts on files that the patch being replayed deletes are resolved by keeping
### the deletion (that is how the upstream build infrastructure stays removed). Any other
### conflict stops the rebase: resolve it, `git add` the files and run
### `hack/rebase-sidero.sh --continue`.

set -eu

# Keep git rebase --continue from opening an editor for every replayed commit message.
export GIT_EDITOR=true

die() {
    echo "error: $*" >&2
    exit 1
}

rebasing() {
    git rev-parse -q --verify REBASE_HEAD > /dev/null
}

conflicts() {
    git diff --name-only --diff-filter=U
}

# Keep the deletion for every file the commit being replayed removes.
resolve_deletions() {
    deleted="$(git diff-tree --no-commit-id --name-only -r --diff-filter=D REBASE_HEAD)"
    if [ -n "${deleted}" ]; then
        echo "${deleted}" | git rm --ignore-unmatch --pathspec-from-file=-
    fi
}

if [ $# -ne 1 ]; then
    die "usage: $0 <tag> | --continue | --abort"
fi

case "$1" in
    --continue)
        set -- rebase --continue
        ;;
    --abort)
        git rebase --abort
        branch="$(git rev-parse --abbrev-ref HEAD)"
        git checkout -
        git branch -D "${branch}"
        exit 0
        ;;
    v*)
        git fetch upstream tag "$1"
        git checkout -b "versions/${1#v}"
        set -- rebase --onto "$1" "$(git merge-base HEAD "$1")"
        ;;
    *)
        die "usage: $0 <tag> | --continue | --abort"
        ;;
esac

until git "$@"; do
    if ! rebasing; then
        exit 1
    fi
    if [ -z "$(conflicts)" ]; then
        die "git $* stopped without conflicts, see above"
    fi
    resolve_deletions
    if [ -n "$(conflicts)" ]; then
        die "resolve the conflicts above, 'git add' them and run: $0 --continue"
    fi
    set -- rebase --continue
done
