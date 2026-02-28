export interface Lyra3Track {
  id: string;
  prompt: string;
  status: 'pending' | 'success' | 'failed';
  url?: string;
}

export interface Lyra3State {
  tracks: Lyra3Track[];
}
