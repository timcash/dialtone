declare module 'autobase' {
    import { EventEmitter } from 'events';

    interface AutobaseOptions {
        inputs: any[];
        localInput?: any;
        localOutput?: any;
        outputs?: any[];
    }

    export default class Autobase extends EventEmitter {
        constructor(options: AutobaseOptions);
        constructor(localInput: any, inputs?: any[]);

        view: any;

        ready(): Promise<void>;
        addInput(input: any): Promise<void>;
        removeInput(input: any): Promise<void>;
        append(value: any): Promise<void>;
        replicate(isInitiator: boolean, options?: any): any;
    }
}
