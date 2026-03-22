import './style.css';
import './app.css';

import logo from './assets/images/logo-universal.png';
import {Greet} from '../wailsjs/go/main/App';
// import 'https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.0-RC.8/bundles/datastar.js';


window.location.href = "http://localhost:1337";

window.greet = async function () {
  const store = document.querySelector('[data-store]').__data;

  store.message = await Greet(store.name);
};