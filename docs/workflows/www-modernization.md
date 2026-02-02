# Documentation: WWW Component Modernization

Use this workflow to standardize Three.js components, implement predictive lazy loading, and configure adaptive section headers.

## STEP 1. Standardize Component Visibility
Apply the `VisibilityMixin` to all Three.js visualization components to ensure consistent animation gating and standardized `AWAKE`/`SLEEP` logging.

```shell
# 1. Import VisibilityMixin in the component (e.g., src/plugins/www/app/src/components/robot.ts)
# 2. Update setVisible(visible: boolean) to call:
#    VisibilityMixin.setVisible(this, visible, "component-name");
```

## STEP 2. Register Predictive Lazy Loading
Configure the `SectionManager` in `main.ts` to handle predictive pre-buffering. This ensures the next section is loaded while the user is still viewing the current one.

```shell
# Register sections in src/plugins/www/app/src/main.ts:
sections.register('s-robot', {
    containerId: 'robot-container',
    load: async () => {
        const { mountRobot } = await import('./components/robot');
        return mountRobot(document.getElementById('robot-container')!);
    },
    header: { visible: true, subtitle: "precision control" }
});
```

## STEP 3. Configure Adaptive Headers
Use the `HeaderConfig` within the section registration to control the global site header on a per-section basis.

```shell
# Hide header for content-heavy pages like About or Docs:
sections.register('s-about', {
    containerId: 'about-container',
    load: async () => { /* ... */ },
    header: { visible: false } 
});
```

## STEP 4. Optimize Integration Tests
Ensure tests are fast and reliable by forcing GPU acceleration and using reactive waiting instead of fixed sleeps.

```shell
# In src/plugins/www/test/test.go:
# 1. Force the --gpu flag in chromedp.Run arguments.
# 2. Replace time.Sleep() with:
#    chromedp.WaitReady("#target-id.is-visible")
```

## STEP 5. Verify & Build
Run the standardized plugin validation suite to ensure performance and correctness.

```shell
./dialtone.sh plugin build www
./dialtone.sh plugin test www
```
