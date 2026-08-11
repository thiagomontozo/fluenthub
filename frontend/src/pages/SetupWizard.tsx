import { useState, type FormEvent } from 'react'
import { Check } from 'lucide-react'
import { useNavigate } from 'react-router-dom'

const steps = ['Welcome', 'School Information', 'Branding', 'Main Administrator', 'First Unit', 'Academic Defaults', 'Approval Policy', 'Finish']

type SetupData = {
  legalName: string; displayName: string; slug: string; email: string; phone: string; website: string
  timezone: string; locale: string; systemTitle: string; primaryColor: string; secondaryColor: string
  accentColor: string; welcomeText: string; administratorName: string; administratorEmail: string
  administratorPassword: string; unitName: string; unitCode: string; minimumPassingScoreScaled: number
  exerciseWeightBasisPoints: number; examWeightBasisPoints: number; minimumAttendanceBasisPoints: number
}

const initial: SetupData = {
  legalName: '', displayName: '', slug: '', email: '', phone: '', website: '', timezone: 'America/Sao_Paulo', locale: 'pt-BR',
  systemTitle: 'Learning Portal', primaryColor: '#4f46e5', secondaryColor: '#0f172a', accentColor: '#f59e0b', welcomeText: 'Welcome to your learning journey.',
  administratorName: '', administratorEmail: '', administratorPassword: '', unitName: 'Online Unit', unitCode: 'ONLINE', minimumPassingScoreScaled: 7000,
  exerciseWeightBasisPoints: 3000, examWeightBasisPoints: 7000, minimumAttendanceBasisPoints: 7000,
}

export function SetupWizard() {
  const [step, setStep] = useState(0)
  const [data, setData] = useState(initial)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const navigate = useNavigate()
  const update = (field: keyof SetupData, value: string | number) => setData(current => ({ ...current, [field]: value }))
  async function submit(event: FormEvent) {
    event.preventDefault()
    if (step < 7) { setStep(value => value + 1); return }
    setBusy(true); setError('')
    try {
      const response = await fetch('/api/v1/setup/complete', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) })
      if (!response.ok) { const body = await response.json() as { error?: { message?: string } }; throw new Error(body.error?.message ?? 'Setup could not be completed.') }
      navigate('/login', { replace: true })
    } catch (cause) { setError(cause instanceof Error ? cause.message : 'Setup could not be completed.') } finally { setBusy(false) }
  }
  return <main className="setup-page"><aside><div className="setup-logo">F</div><h1>Set up your school</h1><p>A focused start. You can refine everything later.</p><ol>{steps.map((name,index)=><li className={index===step?'active':index<step?'done':''} key={name}><span>{index<step?<Check/>:index+1}</span>{name}</li>)}</ol></aside><section><form className="setup-card" onSubmit={submit}><span>Step {step+1} of {steps.length}</span><h2>{steps[step]}</h2>{error&&<div className="form-error" role="alert">{error}</div>}{step===0&&<div className="setup-illustration"><div>F</div><b>A complete language school workspace</b><small>Administration · Teaching · Assessment · Support</small></div>}{step===1&&<div className="setup-fields"><Field label="Legal name" value={data.legalName} onChange={v=>update('legalName',v)}/><Field label="Display name" value={data.displayName} onChange={v=>update('displayName',v)}/><Field label="Slug" value={data.slug} onChange={v=>update('slug',v)}/><Field label="School email" type="email" value={data.email} onChange={v=>update('email',v)}/><Field label="Phone" value={data.phone} onChange={v=>update('phone',v)}/><Field label="Website" value={data.website} onChange={v=>update('website',v)}/></div>}{step===2&&<div className="setup-fields"><Field label="System title" value={data.systemTitle} onChange={v=>update('systemTitle',v)}/><Field label="Welcome text" value={data.welcomeText} onChange={v=>update('welcomeText',v)}/><Field label="Primary color" type="color" value={data.primaryColor} onChange={v=>update('primaryColor',v)}/><Field label="Secondary color" type="color" value={data.secondaryColor} onChange={v=>update('secondaryColor',v)}/><Field label="Accent color" type="color" value={data.accentColor} onChange={v=>update('accentColor',v)}/></div>}{step===3&&<div className="setup-fields"><Field label="Administrator name" value={data.administratorName} onChange={v=>update('administratorName',v)}/><Field label="Administrator email" type="email" value={data.administratorEmail} onChange={v=>update('administratorEmail',v)}/><Field label="Password (12+ characters)" type="password" value={data.administratorPassword} onChange={v=>update('administratorPassword',v)}/></div>}{step===4&&<div className="setup-fields"><Field label="Unit name" value={data.unitName} onChange={v=>update('unitName',v)}/><Field label="Unit code" value={data.unitCode} onChange={v=>update('unitCode',v)}/><Field label="Timezone" value={data.timezone} onChange={v=>update('timezone',v)}/></div>}{step===5&&<div className="setup-fields"><Field label="Locale" value={data.locale} onChange={v=>update('locale',v)}/><p>Reading, writing, listening, speaking, grammar and vocabulary will be created as editable defaults.</p></div>}{step===6&&<div className="setup-fields"><NumberField label="Passing score (scaled)" value={data.minimumPassingScoreScaled} onChange={v=>update('minimumPassingScoreScaled',v)}/><NumberField label="Exercise weight (basis points)" value={data.exerciseWeightBasisPoints} onChange={v=>update('exerciseWeightBasisPoints',v)}/><NumberField label="Exam weight (basis points)" value={data.examWeightBasisPoints} onChange={v=>update('examWeightBasisPoints',v)}/><NumberField label="Attendance minimum (basis points)" value={data.minimumAttendanceBasisPoints} onChange={v=>update('minimumAttendanceBasisPoints',v)}/></div>}{step===7&&<div className="setup-illustration"><Check/><b>Ready to create your school</b><small>The operation is atomic and creates roles, policies and initial skills.</small></div>}<footer><button type="button" className="secondary-button" disabled={step===0||busy} onClick={()=>setStep(value=>value-1)}>Back</button><button className="primary-button" disabled={busy}>{busy?'Creating school…':step===7?'Finish setup':'Continue'}</button></footer></form></section></main>
}

function Field({label,value,onChange,type='text'}:{label:string;value:string;onChange:(value:string)=>void;type?:string}) { return <label>{label}<input required={label!=='Phone'&&label!=='Website'} minLength={type==='password'?12:undefined} type={type} value={value} onChange={event=>onChange(event.target.value)}/></label> }
function NumberField({label,value,onChange}:{label:string;value:number;onChange:(value:number)=>void}) { return <label>{label}<input required type="number" min="0" max="10000" value={value} onChange={event=>onChange(Number(event.target.value))}/></label> }
