import React from 'react'
import ReactDOM from 'react-dom/client'
import { SwapLook } from './application/swaplook'
import { HttpGateway } from './infrastructure/httpGateway'
import { App } from './presentation/App'
import './presentation/styles.css'

const app = new SwapLook(new HttpGateway())
ReactDOM.createRoot(document.getElementById('root')!).render(<React.StrictMode><App app={app}/></React.StrictMode>)
