import { useState } from 'react'
import Alerts from './components/Alerts'
import DetectionDetail from './components/DetectionDetail'
import NodeHealth from './components/NodeHealth'
import Stats from './components/Stats'
import './App.css'

const TABS = ['Alerts', 'Node Health', 'Stats']

export default function App() {
  const [tab, setTab] = useState('Alerts')
  const [selectedId, setSelectedId] = useState(null)

  function handleSelect(id) {
    setSelectedId(id)
  }

  function handleBack() {
    setSelectedId(null)
  }

  function renderContent() {
    if (tab === 'Alerts') {
      if (selectedId != null) {
        return <DetectionDetail id={selectedId} onBack={handleBack} />
      }
      return <Alerts onSelect={handleSelect} />
    }
    if (tab === 'Node Health') return <NodeHealth />
    if (tab === 'Stats') return <Stats />
  }

  return (
    <div className='app'>
      <header className='header'>
        <h1>SentinelMesh</h1>
        <nav className='tabs'>
          {TABS.map(t => (
            <button
              key={t}
              className={tab === t ? 'tab active' : 'tab'}
              onClick={() => { setTab(t); setSelectedId(null) }}
            >
              {t}
            </button>
          ))}
        </nav>
      </header>
      <main className='content'>
        {renderContent()}
      </main>
    </div>
  )
}
