# Documentation: WWW Development Workflow

This workflow guides an LLM agent through the lifecycle of making changes to the Dialtone `www` site, from branch creation to production deployment.

## STEP 1: Branch Isolation
Always start from a clean `main` branch and create a descriptive feature branch.

```shell
# 1. Ensure you are on main and up to date
git checkout main
git pull origin main

# 2. Create and switch to a new feature branch
./dialtone.sh branch your-feature-name
```

## STEP 2: Local Development & Visualization
Use specific demo environments to verify Three.js visualizations and UI changes in real-time.

```shell
# For Earth/Globe changes:
./dialtone.sh www earth demo

# For CAD/Mechanical changes:
./dialtone.sh www cad demo

# To build the plugin assets:
./dialtone.sh build www
```

## STEP 3: Implementation & Standardization
Follow the [Modernization Workflow](file:///home/user/dialtone/docs/workflows/www-modernization.md) to ensure components use `VisibilityMixin` and predictive lazy loading.

## STEP 4: Automated Verification
Run the full test suite. Tests are optimized to use the `--gpu` flag for fast rendering.

```shell
./dialtone.sh plugin test www
```

## STEP 5: Create Pull Request
Push your changes and use the GitHub CLI wrapper to create a PR.

```shell
# Stage and commit
git add .
git commit -m "Describe your changes clearly"

# Push to origin
git push origin feature/your-feature-name

# Create the PR
./dialtone.sh github pr create --branch feature/your-feature-name --title "Title" --body "Details"
```

## STEP 6: Merge & Publish
Once the PR is approved, merge it and trigger the production deployment.

```shell
# Merge into main
git checkout main
git merge feature/your-feature-name
git push origin main

# Deploy to dialtone.earth
./dialtone.sh www publish
```
