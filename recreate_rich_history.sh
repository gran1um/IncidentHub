#!/bin/sh

# Clean up previous folders
echo "Cleaning old backup..."
rm -rf backup_clean_code
rm -f files_list.txt

# Backup clean files excluding heavy folders and .git
echo "Creating backup with rsync..."
rsync -a --exclude='node_modules' --exclude='.pnpm-store' --exclude='.playwright-cli' --exclude='backup_clean_code' --exclude='.git' --exclude='.cache' ./ backup_clean_code/

# Reset clean-master branch
echo "Resetting clean-master branch..."
git checkout master
git branch -D clean-master
git checkout --orphan clean-master
git rm -rf .

# Find all files in backup_clean_code, sort them alphabetically
find backup_clean_code -type f | sort > files_list.txt

# Count total files
total_files=$(wc -l < files_list.txt)
echo "Total files to commit: $total_files"

# Spreading ~130 commits over 3 months (90 days)
# Start: March 1, 2026, End: May 28, 2026
# Total seconds in 90 days = 90 * 24 * 3600 = 7776000
batch_size=6
total_commits=$(( (total_files + batch_size - 1) / batch_size ))
seconds_step=$(( 7776000 / total_commits ))

echo "Total commits to create: $total_commits"
echo "Seconds step: $seconds_step"

# March 1, 2026 12:00:00 UTC (epoch)
current_time=1772366400

commit_num=1
files_in_batch=""
batch_count=0

while read -r backup_fp; do
    # Get relative path
    rel_path=${backup_fp#backup_clean_code/}
    
    # Create parent dir and copy file back
    mkdir -p "$(dirname "$rel_path")"
    cp "$backup_fp" "$rel_path"
    
    files_in_batch="$files_in_batch $rel_path"
    batch_count=$((batch_count + 1))
    
    if [ $batch_count -eq $batch_size ]; then
        # Determine message based on files in batch
        msg="chore: update workspace assets and configurations"
        
        # We check files_in_batch for matching components
        case "$files_in_batch" in
            *backend/migrations*)
                msg="feat(db): implement database migrations"
                ;;
            *backend/internal/repository*)
                msg="feat(backend): implement repository database access"
                ;;
            *backend/internal/api*)
                msg="feat(api): implement backend API endpoints"
                ;;
            *backend/internal/security*)
                msg="feat(security): implement security rate limiting and filters"
                ;;
            *backend/internal*)
                msg="feat(backend): implement internal services and core logic"
                ;;
            *frontend/src/features*)
                msg="feat(frontend): implement core UI components"
                ;;
            *frontend/src/pages*)
                msg="feat(frontend): implement page layouts and views"
                ;;
            *frontend/tests*)
                msg="test: implement frontend integration test suites"
                ;;
            *frontend/*)
                msg="feat(frontend): configure frontend layouts and styles"
                ;;
            *docs/*|*docs-web/*)
                msg="docs: update system design and product specifications"
                ;;
            *test-data-lab*)
                msg="feat(test-data): add test-data-lab playground modules"
                ;;
        esac
        
        # Git Add
        git add -A
        
        # Convert epoch to ISO-8601
        git_date=$(date -r "$current_time" "+%Y-%m-%dT%H:%M:%S")
        
        # Git Commit
        GIT_AUTHOR_DATE="$git_date" GIT_COMMITTER_DATE="$git_date" git commit -m "$msg"
        
        # Increment time
        current_time=$((current_time + seconds_step))
        commit_num=$((commit_num + 1))
        
        files_in_batch=""
        batch_count=0
    fi
done < files_list.txt

# Commit remaining files if any
if [ $batch_count -gt 0 ]; then
    git add -A
    git_date=$(date -r "$current_time" "+%Y-%m-%dT%H:%M:%S")
    GIT_AUTHOR_DATE="$git_date" GIT_COMMITTER_DATE="$git_date" git commit -m "chore: final touch-ups and workspace synchronization"
fi

# Cleanup
rm -f files_list.txt
rm -rf backup_clean_code

echo "Finished history recreation!"
