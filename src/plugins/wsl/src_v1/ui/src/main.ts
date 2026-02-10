import './style.css'
import { TableSection } from './components/wsl-table'

const tableEl = document.getElementById('wsl-table');
if (tableEl) {
    const table = new TableSection(tableEl);
    table.mount();
}
