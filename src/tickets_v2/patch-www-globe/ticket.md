# Name: patch-www-globe

# Goal
Replace the current D3-based globe with globe.gl while maintaining all existing functionality (arcs, markers, interactive rotation). Fix any build/import issues encountered.

## SUBTASK: Install Dependencies
- name: install-deps
- description: Install globe.gl and three.js in src/plugins/www/app
- test-command: `./dialtone.sh www build`
- status: done

## SUBTASK: Implement globe.gl replacement
- name: implement-globe
- description: Replace D3 globe logic with globe.gl in src/plugins/www/app/src/main.ts (or equivalent)
- test-command: `./dialtone.sh www build`
- status: done

## SUBTASK: Verify globe aesthetics and functionality
- name: verify-globe
- description: Run dev server and manually verify the globe looks premium and has interactive arcs/markers.
- test-command: `./dialtone.sh www build`
- status: done
