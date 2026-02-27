# UI src_v1 Patterns

```sh
# install fixture UI deps
./dialtone.sh ui src_v1 install

# build fixture UI dist
./dialtone.sh ui src_v1 build

# run fixture dev server (local)
./dialtone.sh ui src_v1 dev

# run fixture dev server and open headed browser on mesh node
./dialtone.sh ui src_v1 dev --browser-node chroma

# run UI src_v1 tests
./dialtone.sh ui src_v1 test

# run tests attached to remote headed browser
./dialtone.sh ui src_v1 test --attach chroma
```

`src/plugins/ui/src_v1/ui` is the shared section shell used by plugin UIs.
Shared template presets now live in:
- `src/plugins/ui/src_v1/ui/templates.ts`

For full reference see:
- `src/plugins/ui/README.md`

## Settings Button List Pattern

Use a `button-list` underlay when a section is primarily a vertical list of controls (for example a Settings section).

### HTML

```html
<section id="settings" class="fullscreen" aria-label="Settings Section" hidden>
  <div class="button-list settings-primary overlay-primary" aria-label="Settings Content"></div>
  <aside class="settings-legend overlay-legend" aria-label="Settings Legend"></aside>
</section>
```

### SectionManager registration

```ts
sections.register('settings', {
  containerId: 'settings',
  load: async () => {
    const { mountSettings } = await import('./components/settings/index');
    const container = document.getElementById('settings');
    if (!container) throw new Error('settings container not found');
    return mountSettings(container);
  },
  overlays: {
    primaryKind: 'button-list',
    primary: '.button-list',
    legend: '.settings-legend',
  },
});
```

### CSS behavior

- `button-list` is an underlay and can be used in both `section.fullscreen` and `section.calculator`.
- In `fullscreen`, the list occupies the full section body.
- In `calculator`, the list stays in row 1 while row 2 can still host a `mode-form` for sections that use it.
