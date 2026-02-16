# UIv2 Library

This directory contains the core UI framework (`ui.ts`) for building modular, section-based web applications within Dialtone plugins.

## Key Features

- **Section Management**: `setupApp` provides `sections` to register and navigate between distinct UI sections.
- **Dynamic Loading**: Sections are loaded asynchronously (`load` function), supporting code splitting.
- **Menu Integration**: `menu` allows easy addition of navigation buttons that link to registered sections.
- **VisualizationControl Interface**: Components implement this interface (`mount` and `dispose` methods) for lifecycle management.

## UI Integration Pattern (Example: Robot Plugin)

When integrating a new UI or component that needs to interact with the backend (e.g., sending commands via NATS), follow this pattern:

### 1. Global NATS Export (main.ts)

To allow UI components (like control buttons) to easily send messages to the NATS backend, export the `NatsConnection` and `JSONCodec` from your plugin's `src/main.ts`:

```typescript
// src/plugins/your-plugin/src_vX/ui/src/main.ts
import { connect, JSONCodec, type NatsConnection } from 'nats.ws';

export let NATS_CONNECTION: NatsConnection | null = null;
export const NATS_JSON_CODEC = JSONCodec();

// ... (rest of your main.ts)

async function connectNATS(initData: any) {
  // ... (connection logic)
  try {
    NATS_CONNECTION = await connect({ servers: [server] });
    // ...
    NATS_CONNECTION.closed().then(() => {
      // ...
    });
  } catch (err) {
    // ...
  }
}
```

### 2. Component Usage (e.g., Controls Component)

Components can then import and use these global exports to publish messages:

```typescript
// src/plugins/your-plugin/src_vX/ui/src/components/controls/index.ts
import { VisualizationControl } from '../../../../../../../libs/ui_v2/types';
import { NATS_CONNECTION, NATS_JSON_CODEC } from '../../main'; // Import from main.ts

class ControlsControl implements VisualizationControl {
  // ...
  private sendCommand(cmd: string, mode?: string) {
    if (!NATS_CONNECTION || !NATS_JSON_CODEC) {
      console.warn('NATS not connected. Command not sent.');
      return;
    }

    const payload: { cmd: string; mode?: string } = { cmd };
    if (mode) {
      payload.mode = mode;
    }

    NATS_CONNECTION.publish('rover.command', NATS_JSON_CODEC.encode(payload));
    console.log(`[Controls] Command sent: ${JSON.stringify(payload)}`);
  }
  // ...
}
```

This pattern facilitates communication with the backend while keeping individual UI components self-contained and focused on their specific rendering and interaction logic.
