import { useId } from 'react'
import { useAppearance } from '../lib/appearance.js'

export default function AppearanceSettings() {
    const { mode, palette, cursor, changePreference } = useAppearance()
    const id = useId()
    return (
        <details className='appearance-settings'>
            <summary>Appearance</summary>
            <div className='appearance-options'>
                <fieldset><legend>Reading light</legend><div className='choice-row'>
                    {['system', 'light','dark'].map((value) => <label className='appearance-choice' key={value}>
                        <input type="radio" name={`${id}-mode`} value={value} checked={mode === value} onChange={() => changePreference('mode', value)}/>
                        <span>{value[0].toUpperCase() + value.slice(1)}</span>
                    </label>)}
                </div></fieldset>
                <fieldset><legend>Color palette</legend><div className='choice-row'>
                    {[['parchment','Parchment'], ['sage', 'Botanical'], ['slate', 'Slate']].map(([value, label]) => <label className={`appearance-choice palette-${value}`} key={value}> 
                        <input type='radio' name={`${id}-palette`} value={value} checked={palette === value} onChange={() => changePreference('palette', value)}/>
                        <span>{label}</span>
                    </label>)}
                </div></fieldset>
                <label className='cursor-choice'><input type="checkbox" checked={cursor ==='pen'} onChange={(event) => changePreference('cursor', event.target.checked ? 'pen':'system')}/>Use a pen-nib pointer</label>
                <p>Text fields keep the familiar text cursor. Your choices are remembered on this browser.</p>
            </div>
        </details>
    )
}