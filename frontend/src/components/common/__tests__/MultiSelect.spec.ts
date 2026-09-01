import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import MultiSelect from '../MultiSelect.vue'

describe('MultiSelect', () => {
  const options = [
    { value: 7, label: 'Economy' },
    { value: 41, label: 'Standard' },
    { value: 99, label: 'Premium' },
  ]

  it('supports selecting multiple options and clearing them', async () => {
    const wrapper = mount(MultiSelect, {
      props: {
        modelValue: [],
        options,
        placeholder: 'All eligible groups',
      },
      attachTo: document.body,
    })

    await wrapper.get('.multi-select-trigger').trigger('click')
    const optionButtons = () => Array.from(document.body.querySelectorAll<HTMLButtonElement>('.multi-select-option'))
    expect(optionButtons()).toHaveLength(3)

    await optionButtons()[0].click()
    await wrapper.setProps({ modelValue: [7] })
    await optionButtons()[1].click()
    await wrapper.setProps({ modelValue: [7, 41] })

    expect(wrapper.emitted('change')).toEqual([[[7]], [[7, 41]]])
    expect(wrapper.emitted('update:modelValue')).toEqual([[[7]], [[7, 41]]])
    expect(wrapper.find('.multi-select-count').text()).toBe('2')

    const clearButton = document.body.querySelector<HTMLButtonElement>('.multi-select-clear')
    expect(clearButton).not.toBeNull()
    clearButton!.click()
    await nextTick()
    expect(wrapper.emitted('change')).toEqual([[[7]], [[7, 41]], [[]]])
    expect(wrapper.emitted('update:modelValue')).toEqual([[[7]], [[7, 41]], [[]]])
    wrapper.unmount()
  })

  it('filters options by the search query', async () => {
    const wrapper = mount(MultiSelect, {
      props: {
        modelValue: [],
        options,
        placeholder: 'All eligible groups',
      },
      attachTo: document.body,
    })

    await wrapper.get('.multi-select-trigger').trigger('click')
    const searchInput = document.body.querySelector<HTMLInputElement>('.multi-select-search-input')
    expect(searchInput).not.toBeNull()
    searchInput!.value = 'prem'
    searchInput!.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()

    expect(Array.from(document.body.querySelectorAll('.multi-select-option'))).toHaveLength(1)
    expect(document.body.querySelector('.multi-select-option')?.textContent).toContain('Premium')
    wrapper.unmount()
  })
})
